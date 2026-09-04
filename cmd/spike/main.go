// Spike: install-candidate resolution for `sat install`.
//
// Every probe here calls the real sources.XLookup functions, so what this
// validates is what sat would actually do. Only the resolution policy -
// pruning, grouping and tie-breaking - lives in the spike, because its
// thresholds are still being tuned.
//
// The pipeline:
//
//  0. COLLECT  - exact-name lookup across every ecosystem, in parallel
//  1. PRUNE    - drop what cannot or should not be installed
//     (no binary, deprecated/yanked/disabled/EOL/archived, dead upstream repo)
//  2. IDENTIFY - resolve every survivor's upstream repository (or, failing
//     that, its canonical homepage) so it can be grouped by project
//  3. GROUP    - collapse survivors by upstream, so "one project delivered
//     five ways" stops looking like five choices. Candidates pruned for
//     "exists but cannot deliver" reasons still join their group as
//     non-delivering witnesses - they count toward corroboration below but
//     are never picked to install from.
//  4. RESOLVE  - one group   -> newest version wins, ties fall to source order
//     many groups -> a dominant, corroborated upstream wins,
//     unmeasurable rivals lose quietly and never veto,
//     otherwise PROMPT
//
// The split in step 4 is the point: "which project did you mean?" is a
// correctness question only the user can settle, while "which delivery
// mechanism?" is a preference question the config order already answers.
//
// This is a test harness, not production: it probes every ecosystem
// regardless of which package managers exist on this machine. Set
// SPIKE_SKIP=brew,system to replay a run against one machine's source mix.
//
// Usage: go run ./cmd/spike <name> [<name>...]
package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/sources"
)

const (
	// starDominanceRatio and starFloor decide when competing upstreams are
	// lopsided enough to pick without asking. The floor matters as much as
	// the ratio: 6 stars against 200 is a 33x gap between two equally
	// obscure projects and must still prompt.
	starDominanceRatio = 10
	starFloor          = 1000

	// minCorroboratingSources is how many independent ecosystems must
	// agree on an upstream before a star lead is allowed to auto-install
	// it. Dominance is not confidence: a single-source winner can lead by
	// any margin simply because the correct answer was never collected.
	// Non-delivering witnesses (see the GROUP step) count toward this.
	minCorroboratingSources = 2
)

// configOrder mirrors internal/config.DefaultInstallOrder, translated into
// this spike's probe names (uv->pypi, gh->github) and with `sat` dropped
// since it is not a registry. flatpak is appended because the real config
// has no entry for it yet.
var configOrder = []string{"brew", "nix", "system", "cargo", "pypi", "npm", "github", "flatpak"}

// lookup is one ecosystem's exact-name lookup.
type lookup func(string) (sources.LookupResult, error)

var probes = map[string]lookup{
	"cargo":   sources.CargoLookup,
	"npm":     sources.NpmLookup,
	"pypi":    sources.PyPILookup,
	"brew":    sources.BrewLookup,
	"nix":     sources.NixLookup,
	"system":  sources.SystemLookup,
	"flatpak": sources.FlatpakLookup,
	"github":  sources.GitHubLookup,
}

// sourceOrder fixes the display order so runs are comparable.
var sourceOrder = []string{"cargo", "npm", "pypi", "brew", "nix", "system", "flatpak", "github"}

// candidate is one ecosystem's lookup result as it moves through the
// pipeline.
type candidate struct {
	source string
	sources.LookupResult

	dropped bool
	dropWhy string

	// witness is true for a dropped candidate that still proves the
	// project exists under this identity (no release assets, no binary
	// exposed by the registry) as opposed to negative evidence (archived,
	// deprecated, dead upstream repo). Witnesses join their group for
	// corroboration but are never selected to deliver from.
	witness bool

	unknownWhy string // why binary provision could not be determined

	repoKey string // canonical upstream identity, filled by identify()
}

// available reports whether this machine can actually deliver from source -
// "is the binary on PATH" is necessary but not sufficient, so this must
// mirror what the real XInstall functions actually do, not just probe
// PATH. Known up front (no network), it is applied only at the layer-4
// delivery step - collection and grouping still query every ecosystem
// regardless, since an absent package manager limits what can be
// INSTALLED, never what can be LEARNED (see identify() and the
// corroboration gate, which both need evidence from ecosystems this
// machine may not have). Installability is computed live, never cached:
// `sat source` can change it mid-session.
func available(source string) bool {
	switch source {
	case "system":
		switch common.GetPkgManager() {
		case common.PkgManagerPacman, common.PkgManagerAPT, common.PkgManagerDNF, common.PkgManagerAPK:
			return true
		}
		return false
	case "nix":
		// NixInstall (internal/sources/nix.go) refuses outright on NixOS -
		// packages there are meant to be managed declaratively, not via
		// nix-env - so nix is never a real deliverer on that machine even
		// though the `nix` binary is always on PATH there. Read the
		// distro family from the already-populated os-info cache instead
		// of duplicating nix.go's own /etc/os-release parsing.
		if common.GetDistroFamily() == common.DistroFamilyNixOS {
			return false
		}
		_, err := exec.LookPath("nix-env")
		return err == nil
	case "pypi":
		_, err := exec.LookPath("uv")
		return err == nil
	case "github", "sat":
		return true // release download only needs curl, assumed present
	default:
		_, err := exec.LookPath(source)
		return err == nil
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/spike <name> [<name>...]")
		os.Exit(1)
	}
	if err := common.EnsureOSInfo(); err != nil && os.Getenv(common.EnvSATDebug) != "" {
		fmt.Fprintf(os.Stderr, "[debug] OS detection failed: %v\n", err)
	}

	queries := os.Args[1:]
	verdicts := make([]string, 0, len(queries))
	counts := map[string]int{}

	for i, query := range queries {
		if i > 0 {
			fmt.Println()
		}
		res := run(query)
		verdicts = append(verdicts, formatVerdict(query, res))
		counts[res.rule]++
	}

	printSummary(verdicts, counts)
}

func run(query string) resolution {
	cands := collect(query)
	identify(cands)
	survivors := report(query, cands)
	res := resolve(survivors)
	printResolution(res)
	return res
}

// skippedSources lets a run exclude ecosystems (SPIKE_SKIP=brew,npm).
// This is NOT the production availability gate - it exists so a run can be
// replayed against the source mix of a specific machine, since which
// source "wins" is meaningless if the winner isn't installed there.
var skippedSources = sync.OnceValue(func() map[string]bool {
	out := map[string]bool{}
	for _, s := range strings.Split(os.Getenv("SPIKE_SKIP"), ",") {
		if s = strings.TrimSpace(s); s != "" {
			out[s] = true
		}
	}
	return out
})

// collect fans out to every ecosystem in parallel, applies the local prune
// rules, and returns candidates in display order.
func collect(query string) []candidate {
	var mu sync.Mutex
	var wg sync.WaitGroup
	out := map[string]candidate{}
	skip := skippedSources()

	for source, probe := range probes {
		if skip[source] {
			continue
		}
		wg.Add(1)
		go func(source string, probe lookup) {
			defer wg.Done()
			res, err := probe(query)
			if err != nil {
				return
			}
			mu.Lock()
			out[source] = prune(candidate{source: source, LookupResult: res})
			mu.Unlock()
		}(source, probe)
	}

	wg.Wait()

	ordered := make([]candidate, 0, len(out))
	for _, source := range sourceOrder {
		if c, ok := out[source]; ok {
			ordered = append(ordered, c)
		}
	}
	return ordered
}

// prune marks candidates that cannot or should not be installed. It never
// promotes anything - a surviving candidate is merely not disqualified,
// which is all this stage claims.
//
// Only the GitHub dead-reasons are ever flagged witness: "no releases" and
// "no binary assets" are reported against the repo the candidate already
// resolved to, so they are provably the same project, just non-delivering.
// A bare "provides no binary" from npm/pypi/cargo carries no such guarantee
// - the exact-name match can be a same-named, wholly unrelated project (a
// squatter), so it stays negative evidence rather than being promoted into
// group contention.
func prune(c candidate) candidate {
	switch {
	case c.Dead:
		c.dropped, c.dropWhy = true, c.DeadReason
		c.witness = isWitnessReason(c.DeadReason)
	case c.BinsKnown && len(c.Bins) == 0:
		c.dropped, c.dropWhy = true, "provides no binary"
	case !c.BinsKnown:
		c.unknownWhy = "binary provision not exposed by this registry"
	}
	return c
}

// isWitnessReason reports whether a GitHub dead-reason still proves the
// project exists (no releases published, or a release with no binary
// assets) rather than being negative evidence (archived).
func isWitnessReason(reason string) bool {
	return reason == "no releases" || strings.Contains(reason, "no binary assets")
}

// ---------- identity ----------

// repoInfo caches one repository's star count / existence, shared between
// the dead-upstream prune and the star comparison so each repo is queried
// at most once per run.
type repoInfo struct {
	stars    int
	ok       bool
	notFound bool
}

var repoCache sync.Map // string (owner/repo) -> repoInfo

func getRepoInfo(repo string) repoInfo {
	if v, ok := repoCache.Load(repo); ok {
		return v.(repoInfo)
	}
	stars, ok, notFound := sources.GitHubRepoStars(repo)
	info := repoInfo{stars: stars, ok: ok, notFound: notFound}
	repoCache.Store(repo, info)
	return info
}

// identify resolves every candidate's upstream repository key in place and
// applies the dead-upstream-repo prune: a repo that confirmed-404s is not
// merely unmeasurable, it is gone, and is dropped as negative evidence
// rather than kept as a witness or left to veto star comparisons later.
//
// Registries disagree about what "upstream" means: cargo and npm report a
// repository, while nixpkgs, Flathub and many distro indices report only a
// project homepage. Resolving on the raw value would split one project
// into several, so homepages are resolved to repositories first, and
// repo-less homepages are canonicalized through their HTTP redirect target
// so mirrors of the same site (discordapp.com -> discord.com) collapse
// into one identity.
func identify(cands []candidate) {
	siteToRepo := map[string]string{}

	// Pass 1: candidates that already name a repository need no work, and
	// every other URL they carry becomes an alias for it.
	for i, c := range cands {
		repo := directRepo(c)
		if repo == "" {
			continue
		}
		cands[i].repoKey = repo
		for _, alias := range identityHints(c) {
			siteToRepo[alias] = repo
		}
	}

	// Pass 2: everything else is identified by a homepage-shaped URL.
	// Registries are inconsistent about which field it lands in - Arch
	// publishes the project site in its `url` field, nixpkgs has only
	// package_homepage - so both are tried.
	for i, c := range cands {
		if cands[i].repoKey != "" {
			continue
		}

		hints := identityHints(c)
		if len(hints) == 0 {
			continue // nothing identifies this candidate at all
		}

		for _, hint := range hints {
			if repo, ok := siteToRepo[hint]; ok {
				cands[i].repoKey = repo
				break
			}
		}
		if cands[i].repoKey != "" {
			continue
		}

		// Nothing else knew this site, so pay for a fetch and read the
		// repository off the project's own page.
		for _, raw := range []string{c.Homepage, c.Repo} {
			if raw == "" {
				continue
			}
			repo, err := sources.RepoFromHomepage(raw)
			if err != nil || repo == "" {
				continue
			}
			cands[i].repoKey = repo
			for _, hint := range hints {
				siteToRepo[hint] = repo
			}
			break
		}
		if cands[i].repoKey != "" {
			continue
		}

		// Still nothing: follow the homepage's own redirect chain. A site
		// that republishes under a second domain (discordapp.com ->
		// discord.com) merges back with whichever candidate already
		// canonicalized to the redirect target.
		if final, err := sources.ResolveRedirect("https://" + hints[0]); err == nil && final != "" {
			cands[i].repoKey = final
			siteToRepo[hints[0]] = final
			continue
		}

		cands[i].repoKey = hints[0]
	}

	// Dead-upstream-repo prune: a candidate whose repo key is a GitHub
	// repository that confirmed-404s is not in contention at all, and it
	// is negative evidence, not a witness - it never founds or corroborates
	// a group.
	for i, c := range cands {
		if c.dropped && !c.witness {
			continue // already excluded, no need to spend a request
		}
		key := c.repoKey
		if !sources.IsRepoURL(key) {
			continue
		}
		info := getRepoInfo(strings.TrimPrefix(key, "github.com/"))
		if info.notFound {
			cands[i].dropped, cands[i].witness = true, false
			cands[i].dropWhy = "repo not found (404)"
		}
	}
}

// directRepo returns the candidate's repository URL if any field already
// holds one. Which field that is varies by registry: nixpkgs has no repo
// field at all and frequently puts the repository in package_homepage,
// while Arch puts a project website in the field named `url`. Trusting
// only the field name would orphan whichever registry disagrees.
func directRepo(c candidate) string {
	for _, raw := range []string{c.Repo, c.Homepage} {
		if canonical := sources.CanonicalRepo(raw); sources.IsRepoURL(canonical) {
			return canonical
		}
	}
	return ""
}

// identityHints returns the canonical site-shaped URLs a candidate offers
// as evidence of which project it is, excluding real repository URLs
// (which are handled directly).
func identityHints(c candidate) []string {
	var hints []string
	for _, raw := range []string{c.Homepage, c.Repo} {
		canonical := sources.CanonicalRepo(raw)
		if canonical == "" || sources.IsRepoURL(canonical) {
			continue
		}
		hints = append(hints, canonical)
	}
	return hints
}

// ---------- grouping ----------

// group is one distinct upstream project and every ecosystem delivering or
// witnessing it.
type group struct {
	upstream string
	members  []candidate
	stars    int
	starsOK  bool // false when the star count could not be read at all
}

// groupByUpstream collapses candidates that resolve to the same project.
// Candidates are pre-identified by identify(), so this is a pure grouping
// pass keyed on repoKey.
func groupByUpstream(cands []candidate) []group {
	var groups []group
	index := map[string]int{}

	for _, c := range cands {
		key := c.repoKey
		if key == "" {
			key = "unknown (" + c.source + ")"
		}
		if at, ok := index[key]; ok {
			groups[at].members = append(groups[at].members, c)
			continue
		}
		index[key] = len(groups)
		groups = append(groups, group{upstream: key, members: []candidate{c}})
	}

	return groups
}

// enrichMissingGitHub closes the gh-class gap: GitHubLookup only searches
// by name, so a project whose repo name differs from its binary name (the
// GitHub CLI ships `gh` from `cli/cli`) is invisible to it even after
// brew/nix/a homepage redirect have already told the resolver which repo
// it lives at. Once GROUP has settled a project's identity, any group
// still missing a GitHub member gets one real API call to fetch that
// repo's release assets directly by path - the difference between
// correctly resolving `cli/cli` and reporting NO DELIVERER for a project
// that in fact ships ready binaries.
//
// This deliberately does NOT rescue every unmeasurable case: `nodejs/node`
// genuinely has 0 release assets, so a project like that still and
// correctly ends up NO DELIVERER.
func enrichMissingGitHub(groups []group) {
	for i := range groups {
		g := &groups[i]
		if !sources.IsRepoURL(g.upstream) {
			continue
		}

		hasGitHub := false
		for _, m := range g.members {
			if m.source == "github" {
				hasGitHub = true
				break
			}
		}
		if hasGitHub {
			continue
		}

		repo := strings.TrimPrefix(g.upstream, "github.com/")
		res, err := sources.GitHubLookupByRepo(repo)
		if err != nil {
			continue
		}

		c := prune(candidate{source: "github", LookupResult: res})
		c.repoKey = g.upstream
		status := "KEEP"
		if c.dropped {
			status = "DROP"
		}
		fmt.Printf("  github    %-5s %s %s  (found by repo path, not by name)\n", status, c.Name, c.Version)

		if c.dropped && !c.witness {
			continue // negative evidence (archived etc.), doesn't join the group
		}
		g.members = append(g.members, c)
	}
}

// deliverable returns the members of a group that can actually be
// installed from - witnesses prove the project exists but ship nothing.
//
// Machine availability (is cargo/nix/brew/flatpak actually installed
// here?) is NOT applied in this spike - it deliberately probes every
// ecosystem regardless of the local machine's source mix (see SPIKE_SKIP).
// Real availability filtering belongs where install.go already knows what
// is installed, once this pipeline moves into internal/resolver (step 8).
//
// The GUI delivery preference (nix last, brew second-last for desktop
// apps) is NOT applied here - see isDesktopGroup / desktopConfigOrder,
// used by resolveWithinGroup. An early version of this function deleted
// nix outright, which meant a GUI app carried ONLY by nix reported NO
// DELIVERER instead of installing. Reordering degrades gracefully;
// exclusion does not.
func deliverable(members []candidate) []candidate {
	out := make([]candidate, 0, len(members))
	for _, m := range members {
		if !m.witness && available(m.source) {
			out = append(out, m)
		}
	}
	return out
}

// isDesktopGroup reports whether any member confirms this project is a GUI
// application (Flathub's authoritative `type` field).
func isDesktopGroup(members []candidate) bool {
	for _, m := range members {
		if m.DesktopAppKnown && m.IsDesktopApp {
			return true
		}
	}
	return false
}

// desktopConfigOrder is configOrder with nix and brew demoted to last and
// second-last. The objection to nix for GUI apps is brokenness (no desktop
// integration, sandboxing quirks), not staleness, and to brew to a lesser
// degree for the same reason - so a marginally newer nix or brew build
// must never win a GUI app on freshness. Both stay reachable as a last
// resort rather than being excluded, so a GUI app carried only by nix
// still installs from nix.
var desktopConfigOrder = func() []string {
	order := make([]string, 0, len(configOrder))
	for _, s := range configOrder {
		if s != "nix" && s != "brew" {
			order = append(order, s)
		}
	}
	return append(order, "brew", "nix")
}()

// ---------- resolution ----------

// resolution is the pipeline's verdict for one query.
type resolution struct {
	winner *candidate
	rule   string
	detail string
	groups []group // populated when the rule is rulePrompt
	why    string  // why the pipeline refused to decide
}

const (
	ruleNone        = "no candidates"
	ruleOnly        = "only candidate"
	ruleFreshness   = "freshness"
	ruleOrder       = "source order"
	ruleStars       = "stars"
	rulePrompt      = "PROMPT"
	ruleNoDeliverer = "no deliverer" // resolved project, nothing can install it (step 7 handles this properly)
)

func resolve(cands []candidate) resolution {
	groups := groupByUpstream(cands)
	enrichMissingGitHub(groups)

	switch len(groups) {
	case 0:
		return resolution{rule: ruleNone}
	case 1:
		return resolveWithinGroup(groups[0])
	default:
		return resolveAcrossGroups(groups)
	}
}

// resolveWithinGroup answers "which delivery mechanism?". Every member
// installs the same project, so any deliverable answer is correct - prefer
// whichever ships the newest release, and let the configured order settle
// the common case where they all ship the same version. A group made up
// entirely of witnesses has nothing that can actually be installed.
func resolveWithinGroup(g group) resolution {
	members := deliverable(g.members)
	if len(members) == 0 {
		return resolution{rule: ruleNoDeliverer,
			detail: fmt.Sprintf("resolved to %s, carried only by non-delivering source(s) %s - run `sat source` to add one",
				g.upstream, sourceNames(g.members))}
	}
	if len(members) == 1 {
		winner := members[0]
		return resolution{winner: &winner, rule: ruleOnly, detail: g.upstream}
	}

	// Desktop apps skip freshness entirely and go straight to the
	// GUI-aware config order: a marginally newer nix or brew build must
	// never win a GUI app back from flatpak (or whatever else is
	// available) on version alone.
	if isDesktopGroup(members) {
		return resolution{
			winner: byConfigOrder(members, desktopConfigOrder),
			rule:   ruleOrder,
			detail: fmt.Sprintf("desktop app, nix/brew deprioritized, among %s", sourceNames(members)),
		}
	}

	if best, ok := newestMember(members); ok {
		return resolution{
			winner: best,
			rule:   ruleFreshness,
			detail: fmt.Sprintf("%s %s ahead of %s", best.source, best.Version, otherVersions(members, best)),
		}
	}

	return resolution{
		winner: byConfigOrder(members, configOrder),
		rule:   ruleOrder,
		detail: fmt.Sprintf("versions tie or are undecidable across %s", sourceNames(members)),
	}
}

// resolveAcrossGroups answers "which project did you mean?" - a
// correctness question, so it only auto-answers when one project is both
// overwhelmingly more established and independently corroborated.
//
// Only measurable groups (GitHub upstreams whose star count was actually
// read) compete for leadership. An unmeasurable group can never win and
// can never veto a decision by its mere presence - it simply loses
// quietly, because "we don't know its popularity" is not evidence against
// it. This matters because a single repo-less orphan (a pypi sdist with no
// homepage, say) must not be able to force a prompt on an otherwise
// obviously-decided query.
func resolveAcrossGroups(groups []group) resolution {
	for i := range groups {
		groups[i].stars, groups[i].starsOK = upstreamStars(groups[i].upstream)
	}

	measurable := make([]group, 0, len(groups))
	for _, g := range groups {
		if g.starsOK {
			measurable = append(measurable, g)
		}
	}
	sort.SliceStable(measurable, func(i, j int) bool { return measurable[i].stars > measurable[j].stars })

	if len(measurable) == 0 {
		return resolution{rule: rulePrompt, groups: groups,
			why: "no candidate upstream could be measured"}
	}

	leader := measurable[0]

	if leader.stars < starFloor {
		return resolution{rule: rulePrompt, groups: groups,
			why: fmt.Sprintf("leader has %d stars, below the %d floor", leader.stars, starFloor)}
	}

	// Dominance is only checked against the best other MEASURABLE group.
	// Unmeasurable rivals are excluded here on purpose - they lose
	// quietly, they never veto.
	if len(measurable) > 1 {
		runnerUp := measurable[1]
		if runnerUp.stars > 0 && leader.stars < runnerUp.stars*starDominanceRatio {
			return resolution{rule: rulePrompt, groups: groups,
				why: fmt.Sprintf("%d vs %d is under %dx", leader.stars, runnerUp.stars, starDominanceRatio)}
		}
	}

	// Corroboration gate: a lead is only trustworthy if more than one
	// ecosystem independently landed on the same upstream. A single-source
	// winner may lead by a huge margin purely because the right answer was
	// eliminated before scoring - exactly how npm's `gh` won by 284x.
	// Non-delivering witnesses count here: they prove independent
	// ecosystems recognize the same project even if they can't ship it.
	if len(leader.members) < minCorroboratingSources {
		return resolution{rule: rulePrompt, groups: groups,
			why: fmt.Sprintf("leader %s backed by only %s", leader.upstream, sourceNames(leader.members))}
	}

	runnerUpStars, ratio := 0, "∞"
	if len(measurable) > 1 {
		runnerUpStars = measurable[1].stars
		if runnerUpStars > 0 {
			ratio = fmt.Sprintf("%dx", leader.stars/runnerUpStars)
		}
	}

	// Stars only chose the project; which delivery mechanism to use is
	// still the within-group question, so it runs the same rules.
	inner := resolveWithinGroup(leader)
	if inner.rule == ruleNoDeliverer {
		return inner
	}
	return resolution{
		winner: inner.winner,
		rule:   ruleStars,
		detail: fmt.Sprintf("%s ★%d (via %s) vs runner-up ★%d, %s; then %s (%s)",
			leader.upstream, leader.stars, sourceNames(leader.members), runnerUpStars, ratio, inner.rule, inner.detail),
	}
}

// upstreamStars reads a group's star count via the shared repo cache. A
// non-GitHub upstream is not an error - it simply has no star count - but
// it is still reported as unmeasurable so it can never be silently
// outranked.
func upstreamStars(upstream string) (int, bool) {
	rest, ok := strings.CutPrefix(upstream, "github.com/")
	if !ok {
		return 0, false
	}
	info := getRepoInfo(rest)
	return info.stars, info.ok
}

// newestMember returns the single member with the highest version, and
// whether one exists. It reports false when any version is unparseable or
// the highest is shared: both mean freshness has nothing to say and order
// should decide.
func newestMember(members []candidate) (*candidate, bool) {
	best, tied := 0, false

	for i := 1; i < len(members); i++ {
		cmp, ok := common.CompareVersions(members[i].Version, members[best].Version)
		if !ok {
			return nil, false
		}
		switch {
		case cmp > 0:
			best, tied = i, false
		case cmp == 0:
			tied = true
		}
	}

	if tied {
		return nil, false
	}
	return &members[best], true
}

// byConfigOrder picks the member whose source comes first in the user's
// configured install order.
func byConfigOrder(members []candidate, order []string) *candidate {
	bestRank, best := len(order), &members[0]

	for i := range members {
		for rank, source := range order {
			if source == members[i].source && rank < bestRank {
				bestRank, best = rank, &members[i]
			}
		}
	}
	return best
}

// ---------- reporting ----------

// report prints every candidate's final KEEP/DROP status and returns the
// ones that should participate in grouping - deliverable candidates and
// non-delivering witnesses alike, since witnesses still count toward
// corroboration.
func report(query string, cands []candidate) []candidate {
	fmt.Printf("── %s ──\n\n", query)

	var survivors []candidate
	for _, c := range cands {
		if c.dropped && !c.witness {
			fmt.Printf("  %-8s  DROP  %s  (%s)\n", c.source, c.Name, truncate(c.dropWhy, 70))
			continue
		}

		survivors = append(survivors, c)
		status := "KEEP"
		if c.witness {
			status = "KEEP*" // witness: proves the project exists, cannot deliver it
		}
		line := fmt.Sprintf("  %-8s  %s  %s %s", c.source, status, c.Name, c.Version)
		if len(c.Bins) > 0 {
			line += "  bins=" + summarizeBins(c.Bins)
		}
		fmt.Println(line)
		if c.witness {
			fmt.Printf("            witness only: %s\n", c.dropWhy)
		}
		if c.Description != "" {
			fmt.Printf("            %s\n", truncate(c.Description, 88))
		}
		if c.Repo != "" {
			fmt.Printf("            repo: %s\n", c.Repo)
		}
		if c.Homepage != "" {
			fmt.Printf("            home: %s\n", c.Homepage)
		}
	}

	fmt.Println()
	return survivors
}

func printResolution(res resolution) {
	fmt.Println("  " + formatVerdict("", res))

	if res.rule != rulePrompt {
		return
	}
	for i, g := range res.groups {
		stars := "★?"
		if g.starsOK {
			stars = fmt.Sprintf("★%d", g.stars)
		}
		fmt.Printf("       [%d] %-40s %-9s via %s\n", i+1, g.upstream, stars, sourceNames(g.members))
		if d := g.members[0].Description; d != "" {
			fmt.Printf("           %s\n", truncate(d, 82))
		}
	}
}

func formatVerdict(query string, res resolution) string {
	prefix := "=> "
	if query != "" {
		prefix = "=> " + query + " -> "
	}

	switch res.rule {
	case ruleNone:
		return prefix + "nothing installable   [" + ruleNone + "]"
	case rulePrompt:
		return fmt.Sprintf("%sPROMPT (%d choices)   [%s: %s]", prefix, len(res.groups), rulePrompt, res.why)
	case ruleNoDeliverer:
		return fmt.Sprintf("%sNO DELIVERER   [%s: %s]", prefix, ruleNoDeliverer, res.detail)
	default:
		return fmt.Sprintf("%s%s (%s %s)   [%s: %s]",
			prefix, res.winner.source, res.winner.Name, res.winner.Version, res.rule, res.detail)
	}
}

// printSummary reports how often the pipeline decided on its own and which
// rule decided - the number that determines whether this design actually
// feels like `apt install`.
func printSummary(verdicts []string, counts map[string]int) {
	rule := strings.Repeat("═", 78)
	fmt.Printf("\n%s\n  SUMMARY\n%s\n\n", rule, rule)

	for _, v := range verdicts {
		fmt.Println("  " + v)
	}

	total := len(verdicts)
	prompts := counts[rulePrompt]
	fmt.Printf("\n  auto-resolved: %d/%d      prompted: %d/%d\n\n  decided by rule:\n",
		total-prompts, total, prompts, total)

	for _, name := range []string{ruleOnly, ruleFreshness, ruleOrder, ruleStars, rulePrompt, ruleNoDeliverer, ruleNone} {
		if n := counts[name]; n > 0 {
			fmt.Printf("    %-14s %d\n", name, n)
		}
	}
}

func sourceNames(members []candidate) string {
	names := make([]string, 0, len(members))
	for _, m := range members {
		name := m.source
		if m.witness {
			name += "*"
		}
		names = append(names, name)
	}
	return strings.Join(names, "/")
}

// otherVersions renders the losing versions so a freshness verdict shows
// its work.
func otherVersions(members []candidate, winner *candidate) string {
	var out []string
	for i := range members {
		if &members[i] == winner {
			continue
		}
		out = append(out, members[i].source+" "+members[i].Version)
	}
	return strings.Join(out, ", ")
}

// summarizeBins keeps the printout readable when a source reports many
// artifacts (a GitHub release routinely ships 20+ per-platform assets).
func summarizeBins(bins []string) string {
	const maxShown = 4
	if len(bins) <= maxShown {
		return strings.Join(bins, ",")
	}
	return fmt.Sprintf("%s,… (%d total)", strings.Join(bins[:maxShown], ","), len(bins))
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(strings.SplitN(s, "\n", 2)[0])
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

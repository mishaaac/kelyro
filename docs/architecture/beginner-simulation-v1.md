# Beginner Simulation v1

`beginner-simulation-v1` is a deterministic, learner-neutral publication
check. It simulates a learner with no implicit domain knowledge; it neither
reads nor changes I-02 student state.

## Inputs and traversal

The service accepts the compiled knowledge graph, visible hierarchy,
vocabulary artifact, concepts, and explicit pack-authored tool uses and
assumptions. It validates every artifact and walks the graph's topological
order. The hierarchy is used only to attach each visited concept to its topic;
it never replaces prerequisite order.

Before introducing each concept, the simulator verifies that:

- every graph prerequisite has already been introduced;
- every non-baseline vocabulary use has already been defined;
- every tool use follows the tool's `introduced_at` concept;
- every declared assumption has been resolved by an earlier concept.

After those checks, the current concept, its vocabulary definitions, tool
introductions, and assumption resolutions become known. A concept cannot
satisfy a need that occurs in that same concept.

## Output and publication gate

The result records one step per concept, the items introduced at each step,
the final known sets, and structured `BeginnerGap` values. Gap kinds are
prerequisite, vocabulary, tool, and assumption. Input order does not affect
the output.

The compiler runs this check after gap scanning and before final review. Any
beginner gap is an error in the `beginner_simulation` review dimension and
therefore rejects publication. The pass has no network or LLM dependency and
does not generate lessons, exercises, or student personalization.

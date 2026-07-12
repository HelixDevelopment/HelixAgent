// Isolation marker (HXC-139): the cli_agents/ tree holds vendored third-party
// reference coding agents (Continue, Roo, Kilo, OpenHands, ...) as nested
// submodules. This go.mod makes the whole tree a separate nested module so its
// vendored/fixture .go files are NOT compiled into dev.helix.agent. It is not
// meant to be built. See workable item HXC-139.
module dev.helix.agent.vendored/cli_agents

go 1.26

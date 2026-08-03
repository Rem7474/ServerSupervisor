export default {
  customSyntax: 'postcss-html',
  rules: {
    // Forces reuse of the --tblr-*/--ss-* design tokens (frontend/CLAUDE.md's
    // "Design system" section) instead of a component quietly reintroducing
    // its own one-off palette. Deliberately just this one rule rather than
    // stylelint-config-standard — that preset's syntax-modernization rules
    // (rgb() over rgba(), % alpha notation, media-feature ranges, ...) are
    // unrelated to design-token drift and would turn this into an unrelated
    // repo-wide CSS-style cleanup.
    'color-no-hex': true,
  },
  overrides: [
    {
      // NetworkGraph.vue renders a cytoscape network-topology canvas: its
      // hex colors are a categorical dataviz legend (edge/node/port types —
      // authelia link, internet-proxy link, tcp/udp/service-node dots), not
      // a UI-chrome state that belongs in the --ss-*/--tblr-* status tokens.
      // Same rationale as this file's existing no-explicit-any exemption in
      // eslint.config.js (cytoscape/d3 visualisation, not app UI).
      files: ['**/components/network/NetworkGraph.vue'],
      rules: {
        'color-no-hex': null,
      },
    },
  ],
}

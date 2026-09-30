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
    // Same drift through the functional notations: rgb()/rgba()/hsl()/hsla()
    // with literal channels. A channel list read from a token —
    // rgba(var(--tblr-primary-rgb), 0.1) — is the sanctioned way to get a
    // translucent token color and stays allowed.
    'declaration-property-value-disallowed-list': {
      '/.*/': ['/(?:rgba?|hsla?)\\(\\s*(?!var\\()/'],
    },
  },
  overrides: [
    {
      // Components that predate the rgb()/hsl() rule. Each one is migrated to
      // the --ss-*/--tblr-* tokens (or given a new token in style.css) and
      // removed from this list; don't add to it.
      files: [
        '**/App.vue',
        '**/components/BulkActionBar.vue',
        '**/components/CommandPalette.vue',
        '**/components/LoadingSkeleton.vue',
        '**/components/NotificationBell.vue',
        '**/components/alerts/AlertRuleModal.vue',
        '**/components/alerts/AlertRuleStepSource.vue',
        '**/components/host/CommandLogPanel.vue',
        '**/components/network/NetworkNodeDetail.vue',
        '**/components/security/DomainDetailsModal.vue',
        '**/components/security/TrafficWorldMap.vue',
        '**/views/AddHostView.vue',
      ],
      rules: {
        'declaration-property-value-disallowed-list': null,
      },
    },
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
        'declaration-property-value-disallowed-list': null,
      },
    },
  ],
}

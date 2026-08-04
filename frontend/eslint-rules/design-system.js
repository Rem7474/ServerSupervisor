// Local ESLint rules enforcing the small subset of frontend/CLAUDE.md's
// "Design system" section that's mechanically checkable without semantic
// ambiguity (a literal forbidden class string, a native dialog call) — see
// the note at the top of that doc's Design system section for what this
// does and does not cover.
//
// Both rules need to see nodes inside a .vue file's <template> block (e.g. a
// static class="..." attribute, or a string literal inside a `:class="..."`
// binding) in addition to its <script>. vue-eslint-parser only dispatches
// template-body nodes (VAttribute, and the Literal/MemberExpression nodes
// inside a VExpressionContainer) to visitors registered through
// `parserServices.defineTemplateBodyVisitor(...)` — a plain object returned
// from `create()` only ever sees the <script> block's Program. In ESLint 9's
// flat-config API those parser services live at `context.sourceCode.
// parserServices`, not the (now-removed) `context.parserServices`.
function defineVisitor(context, visitor) {
  const parserServices = context.sourceCode && context.sourceCode.parserServices
  if (parserServices && typeof parserServices.defineTemplateBodyVisitor === 'function') {
    // Same checks apply verbatim to <script> code, so reuse one visitor for both.
    return parserServices.defineTemplateBodyVisitor(visitor, visitor)
  }
  // Plain .ts/.js file with no template at all.
  return visitor
}

// Word-bounded so e.g. a hypothetical future "btn-info-box" wouldn't false
// positive, but "btn btn-icon btn-sm btn-ghost-info" does.
const FORBIDDEN_CLASS_PATTERNS = [
  {
    re: /\bbtn-xs\b/,
    message:
      "'btn-xs' doesn't exist in Tabler (silently falls back to the browser default size) — use 'btn-sm', or add a real .btn-xs rule to style.css if a 4th size tier is genuinely needed.",
  },
  {
    re: /\bbtn-(?:outline|ghost)-light\b/,
    message:
      "'btn-outline-light'/'btn-ghost-light' read as a light-theme leftover on this dark-only app — use 'btn-outline-secondary'/'btn-ghost-secondary'.",
  },
  {
    re: /\bbtn-(?:outline|ghost)-orange\b/,
    message:
      "orange is reserved for categorical tagging, not a button role — use the danger/success/warning/primary/secondary token matching the actual action.",
  },
  {
    re: /\bbtn-(?:outline-|ghost-)?info\b/,
    message:
      "'btn-info' isn't one of the 5 documented button tokens (danger/success/warning/primary/secondary) — pick the one matching the action.",
  },
]

function checkClassString(context, node, value) {
  if (typeof value !== 'string') return
  for (const { re, message } of FORBIDDEN_CLASS_PATTERNS) {
    if (re.test(value)) {
      context.report({ node, message: `Forbidden Tabler class in "${value}": ${message}` })
    }
  }
}

export const noForbiddenTablerClass = {
  meta: {
    type: 'problem',
    docs: {
      description: 'disallow Tabler button classes documented as forbidden in frontend/CLAUDE.md',
    },
    schema: [],
  },
  create(context) {
    return defineVisitor(context, {
      // Static `class="..."` attribute — vue-eslint-parser represents its
      // value as a VLiteral, a node type the generic `Literal` visitor below
      // does not reach.
      VAttribute(node) {
        if (node.key?.name === 'class' && node.value?.type === 'VLiteral') {
          checkClassString(context, node.value, node.value.value)
        }
      },
      // Covers `:class="isActive ? 'btn-ghost-info' : '...'"` — the bound
      // expression's string literals are regular ESTree Literal nodes.
      Literal(node) {
        checkClassString(context, node, node.value)
      },
    })
  },
}

export const noNativeConfirm = {
  meta: {
    type: 'problem',
    docs: {
      description: 'disallow native window.confirm()/window.alert() in favor of useConfirmDialog()/useGlobalToast()',
    },
    schema: [],
  },
  create(context) {
    return defineVisitor(context, {
      MemberExpression(node) {
        if (
          node.object.type === 'Identifier' &&
          node.object.name === 'window' &&
          node.property.type === 'Identifier' &&
          (node.property.name === 'confirm' || node.property.name === 'alert')
        ) {
          context.report({
            node,
            message: `Native window.${node.property.name}() is forbidden — use useConfirmDialog() for confirmations or useGlobalToast() for messages (see frontend/CLAUDE.md's Modals section).`,
          })
        }
      },
    })
  },
}

export default {
  rules: {
    'no-forbidden-tabler-class': noForbiddenTablerClass,
    'no-native-confirm': noNativeConfirm,
  },
}

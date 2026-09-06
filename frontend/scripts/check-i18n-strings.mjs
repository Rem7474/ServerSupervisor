#!/usr/bin/env node
/**
 * Fails if a source file outside src/locales/ carries hardcoded French
 * user-facing text and isn't listed in src/locales/.i18n-migration-allowlist.txt.
 *
 * Replaces an earlier grep-based check that only matched accented characters in
 * *.vue. That heuristic had two blind spots which let whole un-migrated files
 * through a green CI:
 *   - unaccented French ("Conteneurs", "Projets Compose", "HORS LIGNE") ;
 *   - *.ts entirely — which is where toasts and confirm dialogs live.
 *
 * Detection is AST-based rather than line-based so it can look at *user-facing*
 * strings only: Vue template text nodes, a fixed set of user-visible static
 * attributes, and string literals in script code. Comments are ignored on
 * purpose — French comments are idiomatic in this repo and are not shipped.
 */
import { readFileSync, readdirSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { parse } from 'vue/compiler-sfc'

const FRONTEND_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const SRC = join(FRONTEND_ROOT, 'src')
const ALLOWLIST = join(SRC, 'locales', '.i18n-migration-allowlist.txt')

/** Static attributes whose value is rendered to (or announced at) the user. */
const USER_VISIBLE_ATTRS = new Set([
  'alt', 'aria-description', 'aria-label', 'aria-placeholder', 'aria-roledescription',
  'cancel-text', 'confirm-text', 'description', 'empty-text', 'header', 'label',
  'placeholder', 'text', 'title',
])

/**
 * French *content* words: distinctive enough that a single match is conclusive,
 * even in a one-word string like "Conteneurs". Unaccented spellings are listed
 * too because part of this repo's un-migrated copy is written without accents.
 */
const STRONG_WORDS = [
  'actifs', 'actives', 'affiches', 'ajouter', 'annuler', 'aucun', 'aucune',
  'aujourd', 'charger', 'chargement', 'conteneur', 'conteneurs', 'confirmer',
  'deja', 'echec', 'echoue', 'enregistrer', 'enregistree', 'erreur', 'fermer',
  'fenetre', 'hote', 'hors', 'impossible', 'introuvable', 'irreversible',
  'lancer', 'ligne', 'modifier', 'nouveau', 'nouvelle', 'parametres', 'projets',
  'regle', 'relancer', 'requis', 'requise', 'reussi', 'reussie', 'selectionner',
  'supprimer', 'supprime', 'supprimee', 'tapez', 'utilisateur', 'utilisateurs',
  'veuillez',
]

/**
 * French function words. Individually ambiguous (many are English words, CSS
 * tokens or identifiers), so two distinct matches are required before a string
 * counts as French — which is what "sans-serif" or "Status ou Date" need.
 */
const WEAK_WORDS = [
  'avec', 'ce', 'ces', 'cet', 'cette', 'chaque', 'dans', 'des', 'doit',
  'doivent', 'du', 'elle', 'est', 'et', 'la', 'le', 'les', 'leur', 'lors',
  'mais', 'nom', 'notre', 'ou', 'par', 'pas', 'plusieurs', 'pour', 'sans',
  'ses', 'seul', 'seule', 'son', 'sont', 'sur', 'tous', 'toutes', 'un', 'une',
  'votre', 'vous',
]

const wordRe = (words) => new RegExp(`(?<!\\p{L})(${words.join('|')})(?!\\p{L})`, 'giu')
const STRONG_RE = wordRe(STRONG_WORDS)
const WEAK_RE = wordRe(WEAK_WORDS)
const ACCENT_RE = /[éèàêôûîçëïüùâäöœÉÈÀÊÔÛÎÇËÏÜÙÂÄÖŒ]/u
/** Accented strings that are still not prose: CSS shorthand, units, selectors. */
const CSS_LIKE_RE = /^[\w\s,%#.()-]*\b(serif|sans-serif|monospace|solid|dashed|rgba?|em|rem|px)\b/i

function distinctMatches(re, s) {
  return new Set((s.match(re) ?? []).map((m) => m.toLowerCase())).size
}

/**
 * A string is "French user-facing text" when it reads like prose (not a code
 * token) and carries either a French accent, one strong French content word, or
 * two distinct French function words.
 */
function isFrenchUserText(raw) {
  const s = raw.trim()
  if (s.length < 3) return false
  if (!/\p{L}{2}/u.test(s)) return false
  if (/^https?:\/\//i.test(s)) return false
  // Single-token values carrying code punctuation are identifiers/paths/selectors,
  // never user copy: "foo.bar", "src/x", "text-muted", "#id", "${expr}".
  if (!/\s/.test(s) && /[._/#:[\]{}$\\]|--/.test(s)) return false
  if (CSS_LIKE_RE.test(s)) return false
  if (ACCENT_RE.test(s)) return true
  return distinctMatches(STRONG_RE, s) >= 1 || distinctMatches(WEAK_RE, s) >= 2
}

function walkTemplate(node, hits) {
  if (!node) return
  if (node.type === 2 /* TEXT */) {
    if (isFrenchUserText(node.content)) {
      hits.push({ line: node.loc.start.line, text: node.content.trim() })
    }
  }
  if (node.type === 1 /* ELEMENT */) {
    for (const prop of node.props ?? []) {
      // type 6 = plain (non-bound) attribute; a :bound value is an expression.
      if (prop.type === 6 && prop.value && USER_VISIBLE_ATTRS.has(prop.name)) {
        if (isFrenchUserText(prop.value.content)) {
          hits.push({ line: prop.loc.start.line, text: `${prop.name}="${prop.value.content.trim()}"` })
        }
      }
    }
  }
  for (const child of node.children ?? []) walkTemplate(child, hits)
}

/**
 * Strips comments and template-literal expression holes, then yields every
 * string/template literal with its line number. Good enough for a lint guard:
 * it errs toward *not* reporting rather than misreporting.
 */
function* scriptLiterals(code) {
  const withoutComments = code
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/(^|[^:])\/\/[^\n]*/g, (m, p) => p + ' '.repeat(m.length - p.length))
  const re = /(['"`])((?:\\.|(?!\1)[^\\])*)\1/g
  let m
  while ((m = re.exec(withoutComments)) !== null) {
    const line = withoutComments.slice(0, m.index).split('\n').length
    yield { line, text: m[2].replace(/\$\{[^}]*\}/g, '') }
  }
}

function scanFile(absPath) {
  const src = readFileSync(absPath, 'utf8')
  const hits = []
  if (absPath.endsWith('.vue')) {
    const { descriptor, errors } = parse(src, { filename: absPath })
    if (errors.length) return hits
    if (descriptor.template?.ast) walkTemplate(descriptor.template.ast, hits)
    for (const block of [descriptor.script, descriptor.scriptSetup]) {
      if (!block) continue
      const offset = src.slice(0, block.loc.start.offset).split('\n').length - 1
      for (const lit of scriptLiterals(block.content)) {
        if (isFrenchUserText(lit.text)) hits.push({ line: lit.line + offset, text: lit.text.trim() })
      }
    }
  } else {
    for (const lit of scriptLiterals(src)) {
      if (isFrenchUserText(lit.text)) hits.push({ line: lit.line, text: lit.text.trim() })
    }
  }
  return hits
}

function collectSources(dir, out = []) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, entry.name)
    if (entry.isDirectory()) {
      if (entry.name === 'locales') continue
      collectSources(p, out)
      continue
    }
    // Specs assert on French copy by design — they are not shipped to users.
    if (/\.(spec|test)\.[tj]s$/.test(entry.name)) continue
    if (/\.(vue|ts)$/.test(entry.name)) out.push(p)
  }
  return out
}

const allowed = new Set(
  readFileSync(ALLOWLIST, 'utf8')
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => l && !l.startsWith('#'))
)

const offenders = new Map()
const seen = new Set()
for (const file of collectSources(SRC).sort()) {
  const rel = relative(FRONTEND_ROOT, file)
  seen.add(rel)
  if (allowed.has(rel)) continue
  const hits = scanFile(file)
  if (hits.length) offenders.set(rel, hits)
}

const stale = [...allowed].filter((f) => !seen.has(f))

let failed = false

if (offenders.size) {
  failed = true
  console.error('Hardcoded French user-facing text found outside the i18n migration allowlist:\n')
  for (const [file, hits] of offenders) {
    console.error(`  ${file}`)
    for (const h of hits.slice(0, 10)) console.error(`    ${file}:${h.line}  ${h.text.slice(0, 100)}`)
    if (hits.length > 10) console.error(`    … and ${hits.length - 10} more`)
    console.error('')
  }
  console.error('Extract these strings into src/locales/{fr,en}/*.json, or if they are')
  console.error(`genuinely not yet migrated, add the file to ${relative(FRONTEND_ROOT, ALLOWLIST)}.`)
}

if (stale.length) {
  failed = true
  console.error('\nStale entries in the migration allowlist (file no longer exists):')
  for (const f of stale) console.error(`  ${f}`)
}

if (failed) process.exit(1)

console.log('OK: no hardcoded French user-facing text outside the i18n migration allowlist.')

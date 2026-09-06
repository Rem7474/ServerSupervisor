# Internationalisation (i18n)

L'interface est disponible en **français** et en **anglais**. Le français est la
langue de référence : c'est celle dans laquelle les messages sont rédigés en
premier, et celle vers laquelle l'application se rabat quand une traduction
manque.

## Choisir sa langue

Le sélecteur (deux drapeaux) est disponible à deux endroits :

- sur la page de connexion, sous le titre ;
- une fois connecté, dans le menu utilisateur (en haut à droite).

Le choix est stocké dans le `localStorage` du navigateur, sous la clé `locale`.
Il est donc propre à un navigateur et à un appareil : il n'est pas rattaché au
compte et n'est pas synchronisé d'une machine à l'autre.

Sans choix explicite, la langue du navigateur (`navigator.language`) est
utilisée si elle correspond à une langue supportée, sinon le français.

## Ce que la langue change

| Élément | Suit la langue de l'interface |
|---|---|
| Textes de l'interface | oui |
| Dates, heures, nombres, tri alphabétique | oui — via `Intl`, avec la locale complète (`fr-FR` / `en-US`) |
| Dates relatives (« il y a 5 min ») | oui — via dayjs |
| Attribut `lang` de la page | oui |
| Messages d'erreur renvoyés par l'API | oui pour les erreurs migrées ; voir plus bas |
| Noms d'hôtes, tags, conteneurs, chemins | non — ce sont vos données |
| Logs et sorties de commandes des agents | non — texte brut de l'hôte supervisé |

### Cas particulier des erreurs de l'API

Le serveur renvoie, en plus du message, un code stable (`i18nKey`) que
l'interface traduit elle-même. C'est nécessaire parce que le navigateur envoie
son propre en-tête `Accept-Language`, que JavaScript n'a pas le droit de
modifier : sans ce code, une erreur serveur s'afficherait dans la langue du
navigateur et non dans celle choisie dans l'interface.

Tous les sites d'erreur ne sont pas encore migrés. Ceux qui ne le sont pas
renvoient un message rédigé en français, affiché tel quel. Cela concerne
aujourd'hui les modules Proxmox, suivi de releases, découverte réseau et
permissions par hôte.

## Ajouter une langue

Le tour complet représente quelques dizaines de lignes de code et la traduction
de ~2 900 messages. Exemple ci-dessous avec l'espagnol (`es`).

### 1. Créer les fichiers de messages

```bash
cd frontend/src/locales
mkdir es
cp fr/*.json es/
```

Les fichiers sont découpés par domaine (`common`, `nav`, `alerts`, `host`, …).
Gardez exactement les mêmes clés que dans `fr/` : un test échoue si une clé
existe dans une langue et pas dans une autre.

### 2. Créer le module de la langue

`frontend/src/locales/es.ts`, sur le modèle de `en.ts` : un `import` par fichier
JSON, puis un `export default` qui les regroupe. Ce module est ce qui permet à
la langue de tenir dans un seul chunk plutôt qu'un par domaine.

### 3. Déclarer la langue

```ts
// frontend/src/i18n.ts
export const SUPPORTED_LOCALES = ['fr', 'en', 'es'] as const

const LOCALE_TAGS: Record<SupportedLocale, string> = {
  fr: 'fr-FR',
  en: 'en-US',
  es: 'es-ES',   // la région sert aux formats de date et de nombre
}
```

```ts
// frontend/src/locales/index.ts
const LOADERS = {
  en: () => import('./en'),
  es: () => import('./es'),
}
```

```ts
// frontend/src/utils/dayjs.ts
import 'dayjs/locale/es'
```

### 4. Ajouter le drapeau au sélecteur

```ts
// frontend/src/components/LocaleSwitcher.vue
const FLAGS = {
  fr: { labelKey: 'common.languageFrench', flagClass: 'flag-country-fr' },
  en: { labelKey: 'common.languageEnglish', flagClass: 'flag-country-gb' },
  es: { labelKey: 'common.languageSpanish', flagClass: 'flag-country-es' },
}
```

Le libellé d'une langue s'écrit **dans cette langue** (`Español`, pas
« Espagnol ») : quelqu'un bloqué sur une interface qu'il ne lit pas doit
pouvoir reconnaître la sienne. Ajoutez donc `languageSpanish` avec la même
valeur `Español` dans **toutes** les langues.

### 5. Vérifier la règle de pluriel

vue-i18n applique par défaut la règle anglaise (singulier pour 1 uniquement).
Le français met aussi 0 au singulier, d'où la règle déclarée dans `i18n.ts` :

```ts
pluralRules: {
  fr: (choice: number) => (Math.abs(choice) <= 1 ? 0 : 1),
},
```

L'espagnol suit la règle anglaise et n'a donc rien à déclarer. Une langue à
plus de deux formes (russe, polonais, arabe…) demande sa propre fonction et
autant de variantes séparées par `|` dans chaque message concerné.

### 6. Ajouter les messages d'erreur du serveur

Les codes d'erreur vivent dans `server/internal/apperr/catalog.go`, dans une
structure à un champ par langue :

```go
type ErrorMessage struct {
	EN string
	FR string
	ES string
}
```

Chaque entrée de `ErrorCatalog` doit alors fournir les trois. `GetMessage`
choisit la langue à partir de l'en-tête `Accept-Language`.

### 7. Lancer les vérifications

```bash
cd frontend && npm run lint:i18n && npm run test && npm run typecheck
cd ../server && go test ./internal/apperr/
```

## Garde-fous automatiques

Ces vérifications tournent en CI et bloquent la fusion :

| Vérification | Ce qu'elle empêche |
|---|---|
| `npm run lint:i18n` | Un texte français écrit en dur dans un `.vue` ou un `.ts` au lieu d'être extrait ; un `v-html` pointé sur une traduction |
| `src/locales/locales.spec.ts` | Une clé présente dans une langue et absente d'une autre ; un message vide ; des variables `{…}` qui diffèrent entre deux langues ; une forme de pluriel perdue |
| `server/internal/apperr/catalog_test.go` | Un code d'erreur sans entrée au catalogue, une entrée sans traduction dans toutes les langues, ou un code absent de `errors.json` côté interface |

## Conventions

- Une clé se nomme `<domaine>.<clefEnCamelCase>` — `host.deleteHostConfirmTitle`.
  Les clés en `MAJUSCULES_AVEC_UNDERSCORES` reprennent un code d'erreur du
  serveur, celles en `snake_case` un identifiant de métrique ou de module.
- Un texte qui affiche un nombre utilise un message au pluriel
  (`'{count} hôte | {count} hôtes'`) et passe le compteur en troisième argument
  de `t()`, jamais une concaténation.
- Deux compteurs indépendants dans un même message ne fonctionnent pas :
  vue-i18n ne résout qu'une seule forme de pluriel par message. Composez à
  partir de deux messages.
- Les dates, nombres et tris passent par `frontend/src/utils/formatters.ts`,
  jamais par une locale écrite en dur.
- Les commentaires de code restent en français : ils ne sont pas livrés.

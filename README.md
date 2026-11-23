# Fathom

Moteur de recherche textuel indexé par trigrammes pour corpus volumineux (JSON, CSV, SQL, TXT…) sur matériel modeste.

---

## Sommaire

- [Aperçu](#aperçu)
- [Architecture](#architecture)
- [Installation](#installation)
- [Utilisation](#utilisation)
- [Structure du projet](#structure-du-projet)
- [Tests](#tests)
- [Performance](#performance)

---

## Aperçu

Fathom indexe une arborescence de fichiers une seule fois et répond ensuite à des recherches répétées en sous-seconde, même sur des corpus de plusieurs gigaoctets.

Principe : pour chaque fichier, les trigrammes (séquences de 3 octets) présents sont extraits et stockés dans des listes de postings au format Roaring (chunks denses ou creux selon la cardinalité). À la recherche, le pattern est d'abord converti en trigrammes obligatoires — les fichiers qui ne contiennent pas tous ces trigrammes sont éliminés sans lecture. Les survivants sont ensuite scannés ligne par ligne en parallèle.

| Commande | Rôle |
|---|---|
| `fathom index <dir>` | Construit ou met à jour l'index |
| `fathom search <pattern>` | Recherche dans l'index |

---

## Architecture

```
corpus/
  scan.go          walk + pool d'extraction trigram
internal/
  trigram/
    trigram.go     Extract / ExtractReader / Pack / OfString
    denseset.go    Set dense 2 MiB (1 bit par trigramme possible)
    parallel.go    ExtractFile multi-cœur pour gros fichiers
  index/
    format.go      Layout binaire FTH1 (magic + docs + postings Roaring)
    builder.go     Builder en mémoire → Write()
    reader.go      Open() → Index, Postings(), TrigramsByDoc()
  bitset/
    roaring.go     Bitmap Roaring minimal (array/bitmap chunks, And)
  query/
    plan.go        RequiredTrigrams (literal et regex)
    search.go      Search / Count (pool multi-fichiers)
    parallel.go    searchFile / scanSegment (split ligne par CPU)
cmd/
  fathom/main.go   CLI index / search
```

### Format d'index (`FTH1`)

```
magic       [4]byte  "FTH1"
version     uint32   (3 = Roaring posting lists)
numDocs     uint32
docs[]:
    pathLen uint32
    path    []byte
    size    uint64
    modTime int64
numTrigrams uint32
trigrams[]: (clés triées)
    key       uint32
    numChunks uint32
    chunks[]:
        highKey uint16
        kind    byte   (0=array, 1=bitmap)
        payload        (array: count+uint16s / bitmap: 1024×uint64)
```

### Parallélisme

- **Indexation** : pool de goroutines (1 par cœur). Chaque gros fichier est lui-même découpé en sections lues en parallèle, budget CPU partagé entre les deux niveaux pour éviter la sur-souscription.
- **Recherche** : pool multi-fichiers + split interne par segments de lignes pour les fichiers > 64 MiB. Chaque worker ouvre son propre descripteur de fichier (Windows sérialise les lectures concurrentes sur un handle partagé).
- **Indexation incrémentale** : un fichier dont la taille et la date de modification n'ont pas changé réutilise ses trigrammes depuis l'ancien index sans re-lecture.

---

## Installation

```bash
git clone <repo>
cd Fathom-Backend
go build -o fathom ./cmd/fathom
```

Requiert Go 1.21+. Aucune dépendance externe.

---

## Utilisation

### Indexer un répertoire

```bash
# Premier index
fathom index --db fathom.idx /chemin/vers/corpus

# Mise à jour incrémentale (fichiers inchangés réutilisés)
fathom index --db fathom.idx /chemin/vers/corpus
```

### Rechercher

```bash
# Recherche littérale
fathom search --db fathom.idx "SELECT * FROM"

# Expression régulière
fathom search --db fathom.idx --regex "error.*timeout"

# Insensible à la casse
fathom search --db fathom.idx --ignore-case "Warning"

# Compter uniquement (pas d'allocation Match)
fathom search --db fathom.idx --count "TODO"

# Limiter les résultats affichés
fathom search --db fathom.idx --limit 20 "pattern"
# --limit 0 = illimité

# Profil CPU
fathom search --db fathom.idx --cpuprofile out.prof "pattern"
```

Sortie : `chemin:numéro_de_ligne:texte_de_la_ligne`

---

## Structure du projet

```
cmd/fathom/         Point d'entrée CLI
internal/bitset/    Bitmap Roaring minimal
internal/corpus/    Walk + indexation parallèle
internal/index/     Format binaire, Builder, Reader
internal/query/     Planification, recherche, comptage
internal/trigram/   Extraction trigrammes, Set dense
```

---

## Tests

```bash
go test ./...
go test -race ./...
```

| Package | Tests clés |
|---|---|
| `trigram` | `TestExtractMatchesReference` — résultat identique à l'implémentation naïve sur 8 tailles |
| `query` | `TestSearchParallelNAgainstSequential` — parallèle == séquentiel sur 9 cas edge |
| `query` | `TestSearchParallelNRandomized` — 150 corpus aléatoires × 1-8 workers |
| `query` | `TestSearchFileEndToEnd` — round-trip complet index→search |
| `query` | `TestCountMatchesSearchAcrossManyFiles` — Count == len(Search) sur 5 fichiers |

---

## Performance

*(Indicatif — dépend du matériel et du corpus)*

| Opération | Ordre de grandeur |
|---|---|
| Extraction trigrammes | ~2 Go/s par cœur (rolling window, pas de hashing) |
| Indexation 1 Go | quelques secondes (multi-cœur) |
| Mise à jour incrémentale | proportionnel aux fichiers modifiés uniquement |
| Recherche littérale (gros index) | sous-seconde (filtrage trigramme + scan parallèle) |
| Seuil parallélisme fichier | 64 MiB (en dessous : scan séquentiel) |

Le format Roaring stocke les posting lists en chunks de 65 536 valeurs : chunk creux → tableau trié uint16, chunk dense → bitmap 8 KiB. L'intersection AND bitmap-bitmap s'exécute en word-at-a-time (64 bits à la fois).

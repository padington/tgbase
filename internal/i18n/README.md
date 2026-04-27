# internal/i18n

UI string lookup keyed by locale. Translation tables are flat key→string YAML files, one file per locale.

## Responsibility

Load every `*.yaml` in a directory as `<basename>.yaml` → locale bundle. Look up keys with locale fallback to a default; substitute `{placeholder}` tokens at render time.

## Public API

```go
type Locale string  // "en", "ru", ...
type Translator interface {
    T(key string, locale Locale, args map[string]any) string
    Has(key string, locale Locale) bool
}
func Load(dir string, defaultLocale Locale) (Translator, error)
```

## Lookup order

1. `bundles[locale][key]` if locale set
2. `bundles[defaultLocale][key]`
3. The key string itself (so missing translations are visible bugs, not silent empty strings)

## Substitution

`{name}` tokens in the template are replaced via `strings.ReplaceAll`. Unrecognized placeholders survive unchanged.

## Where strings live

- UI strings (prompts, errors, button labels, navigation) → `i18n/<locale>.yaml`, baked into the image.
- Product / category names → NOT here; they live on `products.Product` / `products.Category` (data-driven, runtime-mutable).

## When to edit

- **New UI string** → add to every locale yaml, then reference by key from `Outcome.ReplyKey` or `ctx.Trans.T`.
- **New locale** → drop a `<locale>.yaml` in the i18n dir; the loader picks it up automatically. Add the locale code to the `detectLocale` allowlist if you want auto-detection.
- **Change default fallback behavior** → `lookup` here.

## Dependencies

Standard library + `gopkg.in/yaml.v3`. No internal imports.

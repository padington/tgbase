package products

import "testing"

func TestParseSeedYAML_WrapperWithCategoriesAndProducts(t *testing.T) {
	in := []byte(`categories:
  - id: fruits
    emoji: "🍎"
    name_localized: { en: Fruits }
products:
  - name: Apple
    category: fruits
    fodmap: high
    measure: pieces
    stages: { low: 0.25, medium: 0.5, high: 1.0 }
`)
	cats, prods, err := parseSeedYAML(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cats) != 1 || cats[0].ID != "fruits" || cats[0].Emoji != "🍎" {
		t.Errorf("categories: got %+v", cats)
	}
	if len(prods) != 1 || prods[0].Name != "Apple" || prods[0].Category != "fruits" {
		t.Errorf("products: got %+v", prods)
	}
	if got := prods[0].Stages[StageLow]; got != 0.25 {
		t.Errorf("stage parse: got %v", got)
	}
}

func TestParseSeedYAML_WrapperProductsOnly(t *testing.T) {
	in := []byte(`products:
  - name: Honey
    fodmap: high
    measure: spoons
    stages: { low: 0.5, medium: 1, high: 2 }
`)
	cats, prods, err := parseSeedYAML(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cats) != 0 {
		t.Errorf("categories should be empty, got %+v", cats)
	}
	if len(prods) != 1 || prods[0].Name != "Honey" {
		t.Errorf("products: got %+v", prods)
	}
}

func TestParseSeedYAML_LegacyBareList(t *testing.T) {
	in := []byte(`- name: Apple
  fodmap: high
  measure: pieces
  stages: { low: 0.25, medium: 0.5, high: 1.0 }
- name: Cashews
  fodmap: high
  measure: grams
  stages: { low: 10, medium: 20, high: 30 }
`)
	cats, prods, err := parseSeedYAML(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cats != nil {
		t.Errorf("legacy bare list should yield nil categories, got %+v", cats)
	}
	if len(prods) != 2 || prods[0].Name != "Apple" || prods[1].Name != "Cashews" {
		t.Errorf("products: got %+v", prods)
	}
}

func TestParseSeedYAML_Malformed(t *testing.T) {
	in := []byte("this: is: not: valid")
	if _, _, err := parseSeedYAML(in); err == nil {
		t.Error("expected error on malformed YAML, got nil")
	}
}

func TestParseSeedYAML_Empty(t *testing.T) {
	cats, prods, err := parseSeedYAML(nil)
	if err != nil {
		t.Fatalf("empty input should not error, got %v", err)
	}
	if len(cats) != 0 || len(prods) != 0 {
		t.Errorf("expected empty result, got cats=%+v prods=%+v", cats, prods)
	}
}

func TestParseStored_WrapperJSON(t *testing.T) {
	in := []byte(`{"categories":[{"id":"fruits","emoji":"🍎","name_localized":{"en":"Fruits"}}],"products":[{"name":"Apple","category":"fruits","fodmap":"high","measure":"pieces","stages":{"low":0.25,"medium":0.5,"high":1.0}}]}`)
	cats, prods, err := parseStored(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cats) != 1 || cats[0].ID != "fruits" {
		t.Errorf("categories: got %+v", cats)
	}
	if len(prods) != 1 || prods[0].Name != "Apple" {
		t.Errorf("products: got %+v", prods)
	}
}

func TestParseStored_LegacyFlatArray(t *testing.T) {
	in := []byte(`[{"name":"LegacyApple","fodmap":"high","measure":"pieces","stages":{"low":0.25}}]`)
	cats, prods, err := parseStored(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cats != nil {
		t.Errorf("legacy array should yield nil categories, got %+v", cats)
	}
	if len(prods) != 1 || prods[0].Name != "LegacyApple" {
		t.Errorf("products: got %+v", prods)
	}
}

func TestParseStored_LeadingWhitespaceDispatchesToLegacy(t *testing.T) {
	in := []byte("  \n\t[{\"name\":\"LegacyApple\",\"fodmap\":\"high\",\"measure\":\"pieces\",\"stages\":{\"low\":0.25}}]")
	cats, prods, err := parseStored(in)
	if err != nil {
		t.Fatalf("whitespace-prefixed legacy array should parse, got %v", err)
	}
	if cats != nil || len(prods) != 1 || prods[0].Name != "LegacyApple" {
		t.Errorf("got cats=%+v prods=%+v", cats, prods)
	}
}

func TestParseStored_LeadingWhitespaceDispatchesToWrapper(t *testing.T) {
	in := []byte("  \n\t{\"products\":[{\"name\":\"Apple\",\"fodmap\":\"high\",\"measure\":\"pieces\",\"stages\":{\"low\":0.25}}]}")
	cats, prods, err := parseStored(in)
	if err != nil {
		t.Fatalf("whitespace-prefixed wrapper should parse, got %v", err)
	}
	if len(cats) != 0 || len(prods) != 1 {
		t.Errorf("got cats=%+v prods=%+v", cats, prods)
	}
}

func TestParseStored_Empty(t *testing.T) {
	if _, _, err := parseStored(nil); err == nil {
		t.Error("expected error on empty input, got nil")
	}
}

func TestParseStored_Malformed(t *testing.T) {
	if _, _, err := parseStored([]byte("not json")); err == nil {
		t.Error("expected error on malformed JSON, got nil")
	}
}

func TestParseStored_LegacyAndWrapperEquivalentForProducts(t *testing.T) {
	legacy := []byte(`[{"name":"Apple","fodmap":"high","measure":"pieces","stages":{"low":0.25}}]`)
	wrapped := []byte(`{"products":[{"name":"Apple","fodmap":"high","measure":"pieces","stages":{"low":0.25}}]}`)
	_, lp, err := parseStored(legacy)
	if err != nil {
		t.Fatalf("legacy: %v", err)
	}
	_, wp, err := parseStored(wrapped)
	if err != nil {
		t.Fatalf("wrapped: %v", err)
	}
	if len(lp) != 1 || len(wp) != 1 {
		t.Fatalf("expected 1 product in each, got legacy=%d wrapped=%d", len(lp), len(wp))
	}
	if lp[0].Name != wp[0].Name || lp[0].Stages[StageLow] != wp[0].Stages[StageLow] {
		t.Errorf("legacy %+v != wrapped %+v", lp[0], wp[0])
	}
}

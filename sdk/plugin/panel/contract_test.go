package panel

import "testing"

func TestDefinitionAndInteractionTypes(t *testing.T) {
	d := Definition{Format: Format, ID: "position", TitleKey: "plugin.position.title", Fields: []Field{{ID: "mode", Kind: "string"}}, Components: []Component{{ID: "mode", Kind: "select", TitleKey: "plugin.mode", Field: "mode", Event: "mode.change", Options: []Option{{Value: "precise", LabelKey: "plugin.precise"}}}}}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	e := Event{SessionID: "session", EventID: "request", ComponentID: "mode", Name: "mode.change", Value: "precise"}
	if err := d.ValidateEvent(e); err != nil {
		t.Fatal(err)
	}
	e.Value = "missing"
	if d.ValidateEvent(e) == nil {
		t.Fatal("unknown option accepted")
	}
	e.Value = true
	if d.ValidateEvent(e) == nil {
		t.Fatal("wrong input type accepted")
	}
	d.Components = append(d.Components, d.Components[0])
	if d.Validate() == nil {
		t.Fatal("duplicate ID accepted")
	}
}

func TestComponentIconsAreOptionalAndValidated(t *testing.T) {
	d := Definition{Format: Format, ID: "panel", TitleKey: "Panel", Fields: []Field{{ID: "value", Kind: "number"}}, Components: []Component{{ID: "value", Kind: "number", Field: "value", TitleKey: "Value"}}}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	d.Components[0].Icon = "i-tabler-star"
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	d.Components[0].Icon = "https://example.test/icon.svg"
	if err := d.Validate(); err == nil {
		t.Fatal("unsupported icon identifier accepted")
	}
}

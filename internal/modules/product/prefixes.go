package product

// OperatorPrefixInfo defines the operator styling and metadata for mobile clients
type OperatorPrefixInfo struct {
	Name    string `json:"name"`
	Brand   string `json:"brand"`
	Color   string `json:"color"`
	BgColor string `json:"bg_color"`
	Icon    string `json:"icon"`
}

// GetDefaultOperatorPrefixes returns the comprehensive list of Indonesian cellular operator prefixes
func GetDefaultOperatorPrefixes() map[string]OperatorPrefixInfo {
	return map[string]OperatorPrefixInfo{
		// Telkomsel
		"0811": {Name: "Telkomsel", Brand: "Halo", Color: "#EF4444", BgColor: "#FEF2F2", Icon: "cellular"},
		"0812": {Name: "Telkomsel", Brand: "SimPATI", Color: "#EF4444", BgColor: "#FEF2F2", Icon: "cellular"},
		"0813": {Name: "Telkomsel", Brand: "SimPATI", Color: "#EF4444", BgColor: "#FEF2F2", Icon: "cellular"},
		"0821": {Name: "Telkomsel", Brand: "SimPATI", Color: "#EF4444", BgColor: "#FEF2F2", Icon: "cellular"},
		"0822": {Name: "Telkomsel", Brand: "Loop", Color: "#EF4444", BgColor: "#FEF2F2", Icon: "cellular"},
		"0823": {Name: "Telkomsel", Brand: "Kartu As", Color: "#EF4444", BgColor: "#FEF2F2", Icon: "cellular"},
		"0851": {Name: "Telkomsel", Brand: "By.U", Color: "#EF4444", BgColor: "#FEF2F2", Icon: "cellular"},
		"0852": {Name: "Telkomsel", Brand: "Kartu As", Color: "#EF4444", BgColor: "#FEF2F2", Icon: "cellular"},
		"0853": {Name: "Telkomsel", Brand: "Kartu As", Color: "#EF4444", BgColor: "#FEF2F2", Icon: "cellular"},

		// Indosat Ooredoo
		"0814": {Name: "Indosat", Brand: "M2", Color: "#F59E0B", BgColor: "#FFFBEB", Icon: "radio"},
		"0815": {Name: "Indosat", Brand: "Matrix / Mentari", Color: "#F59E0B", BgColor: "#FFFBEB", Icon: "radio"},
		"0816": {Name: "Indosat", Brand: "Matrix / IM3", Color: "#F59E0B", BgColor: "#FFFBEB", Icon: "radio"},
		"0855": {Name: "Indosat", Brand: "Matrix", Color: "#F59E0B", BgColor: "#FFFBEB", Icon: "radio"},
		"0856": {Name: "Indosat", Brand: "IM3", Color: "#F59E0B", BgColor: "#FFFBEB", Icon: "radio"},
		"0857": {Name: "Indosat", Brand: "IM3 Ooredoo", Color: "#F59E0B", BgColor: "#FFFBEB", Icon: "radio"},
		"0858": {Name: "Indosat", Brand: "Mentari Ooredoo", Color: "#F59E0B", BgColor: "#FFFBEB", Icon: "radio"},

		// XL Axiata
		"0817": {Name: "XL Axiata", Brand: "XL", Color: "#2563EB", BgColor: "#EFF6FF", Icon: "wifi"},
		"0818": {Name: "XL Axiata", Brand: "XL", Color: "#2563EB", BgColor: "#EFF6FF", Icon: "wifi"},
		"0819": {Name: "XL Axiata", Brand: "XL", Color: "#2563EB", BgColor: "#EFF6FF", Icon: "wifi"},
		"0859": {Name: "XL Axiata", Brand: "XL", Color: "#2563EB", BgColor: "#EFF6FF", Icon: "wifi"},
		"0877": {Name: "XL Axiata", Brand: "XL Prioritas", Color: "#2563EB", BgColor: "#EFF6FF", Icon: "wifi"},
		"0878": {Name: "XL Axiata", Brand: "XL", Color: "#2563EB", BgColor: "#EFF6FF", Icon: "wifi"},

		// Axis
		"0831": {Name: "AXIS", Brand: "Axis", Color: "#8B5CF6", BgColor: "#F5F3FF", Icon: "flash"},
		"0832": {Name: "AXIS", Brand: "Axis", Color: "#8B5CF6", BgColor: "#F5F3FF", Icon: "flash"},
		"0833": {Name: "AXIS", Brand: "Axis", Color: "#8B5CF6", BgColor: "#F5F3FF", Icon: "flash"},
		"0838": {Name: "AXIS", Brand: "Axis", Color: "#8B5CF6", BgColor: "#F5F3FF", Icon: "flash"},

		// Tri (3)
		"0895": {Name: "Tri (3)", Brand: "Tri", Color: "#EA580C", BgColor: "#FFF7ED", Icon: "planet"},
		"0896": {Name: "Tri (3)", Brand: "Tri", Color: "#EA580C", BgColor: "#FFF7ED", Icon: "planet"},
		"0897": {Name: "Tri (3)", Brand: "Tri", Color: "#EA580C", BgColor: "#FFF7ED", Icon: "planet"},
		"0898": {Name: "Tri (3)", Brand: "Tri", Color: "#EA580C", BgColor: "#FFF7ED", Icon: "planet"},
		"0899": {Name: "Tri (3)", Brand: "Tri", Color: "#EA580C", BgColor: "#FFF7ED", Icon: "planet"},

		// Smartfren
		"0881": {Name: "Smartfren", Brand: "Smartfren", Color: "#EC4899", BgColor: "#FDF2F8", Icon: "hardware-chip"},
		"0882": {Name: "Smartfren", Brand: "Smartfren", Color: "#EC4899", BgColor: "#FDF2F8", Icon: "hardware-chip"},
		"0883": {Name: "Smartfren", Brand: "Smartfren", Color: "#EC4899", BgColor: "#FDF2F8", Icon: "hardware-chip"},
		"0884": {Name: "Smartfren", Brand: "Smartfren", Color: "#EC4899", BgColor: "#FDF2F8", Icon: "hardware-chip"},
		"0885": {Name: "Smartfren", Brand: "Smartfren", Color: "#EC4899", BgColor: "#FDF2F8", Icon: "hardware-chip"},
		"0886": {Name: "Smartfren", Brand: "Smartfren", Color: "#EC4899", BgColor: "#FDF2F8", Icon: "hardware-chip"},
		"0887": {Name: "Smartfren", Brand: "Smartfren", Color: "#EC4899", BgColor: "#FDF2F8", Icon: "hardware-chip"},
		"0888": {Name: "Smartfren", Brand: "Smartfren 4G", Color: "#EC4899", BgColor: "#FDF2F8", Icon: "hardware-chip"},
		"0889": {Name: "Smartfren", Brand: "Smartfren", Color: "#EC4899", BgColor: "#FDF2F8", Icon: "hardware-chip"},
	}
}

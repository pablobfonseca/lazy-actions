package tui

type keyBinding struct {
	Keys []string
	Desc string
}

type keySection struct {
	Title    string
	Bindings []keyBinding
}

var keyMap = []keySection{
	{
		Title: "Navigation",
		Bindings: []keyBinding{
			{Keys: []string{"j", "↓"}, Desc: "move down"},
			{Keys: []string{"k", "↑"}, Desc: "move up"},
			{Keys: []string{"g"}, Desc: "first run"},
			{Keys: []string{"G"}, Desc: "last run"},
		},
	},
	{
		Title: "Actions",
		Bindings: []keyBinding{
			{Keys: []string{"enter"}, Desc: "open log viewer"},
			{Keys: []string{"o"}, Desc: "open in browser"},
			{Keys: []string{"r"}, Desc: "refresh now"},
			{Keys: []string{"F"}, Desc: "re-run failed jobs"},
			{Keys: []string{"x"}, Desc: "cancel run"},
			{Keys: []string{"d"}, Desc: "download artifacts"},
			{Keys: []string{"y"}, Desc: "copy run URL"},
			{Keys: []string{"Y"}, Desc: "copy commit SHA"},
		},
	},
	{
		Title: "Filters",
		Bindings: []keyBinding{
			{Keys: []string{"a"}, Desc: "active only"},
			{Keys: []string{"f"}, Desc: "failed only"},
			{Keys: []string{"A"}, Desc: "all (reset)"},
			{Keys: []string{"/"}, Desc: "fuzzy filter"},
			{Keys: []string{"esc"}, Desc: "clear filter"},
		},
	},
	{
		Title: "Log viewer",
		Bindings: []keyBinding{
			{Keys: []string{"/"}, Desc: "search"},
			{Keys: []string{"n"}, Desc: "next match"},
			{Keys: []string{"N"}, Desc: "previous match"},
			{Keys: []string{"g"}, Desc: "top"},
			{Keys: []string{"G"}, Desc: "bottom"},
			{Keys: []string{"esc"}, Desc: "close"},
		},
	},
	{
		Title: "Global",
		Bindings: []keyBinding{
			{Keys: []string{"?"}, Desc: "help"},
			{Keys: []string{"q", "ctrl+c"}, Desc: "quit"},
		},
	},
}

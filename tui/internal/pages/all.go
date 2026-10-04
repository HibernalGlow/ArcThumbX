package pages

// All builds the page set in navigation order. Adding a screen means one file
// and one line here.
func All(e *Env) []Page {
	return []Page{
		NewDashboard(e),
		NewGeneral(e),
		NewThumbnail(e),
		NewFormats(e),
		NewIntegration(e),
		NewAbout(e),
	}
}

// IndexOf finds a page by ID, defaulting to the first screen. A stale last_page
// preference must not stop the panel from starting.
func IndexOf(pages []Page, id string) int {
	for i, p := range pages {
		if p.ID() == id {
			return i
		}
	}
	return 0
}

// YesNo renders a boolean the way a faceplate does rather than as a bare word.
func YesNo(on bool) string {
	if on {
		return "Enabled"
	}
	return "Disabled"
}

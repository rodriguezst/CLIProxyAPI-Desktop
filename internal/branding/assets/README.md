# Open-door icons

Original vector artwork for CLIProxyAPI Desktop. The approved design uses a
white open door on teal (`#128073`), with slate (`#435068`) for the stopped
Windows tray state. The macOS menu bar uses a black template on transparency;
macOS supplies its appearance in light and dark mode.

The SVGs are the editable sources. Their corresponding PNGs were exported with
CairoSVG at 1024 × 1024 for the launcher variants and 264 × 264 for the menu
template. The Go branding package embeds the PNGs and scales them for each use,
so building and running the app does not require an SVG renderer.

The door knob is transparent in the menu template. Launcher backgrounds include
transparent margins and rounded corners. Windows ICO files contain sizes from
16 through 256 pixels and are embedded in the launcher executable during packaging.

module nimbleterminal.dev/ntgamepad/examples

go 1.26.8

replace nimbleterminal.dev/ntgamepad => ..

// booba's WASM runtime relies on the js/wasm Program wiring provided by the
// neomantra Bubble Tea v2 fork; upstream does not build for js/wasm.
replace charm.land/bubbletea/v2 => github.com/neomantra/bubbletea/v2 v2.0.0-20261020122444-536722b0bdb6

require (
	charm.land/bubbletea/v2 v2.1.1
	charm.land/lipgloss/v2 v2.1.0
	github.com/charmbracelet/x/ansi v0.11.9
	golang.org/x/image v0.48.0
	nimbleterminal.dev/booba v0.13.0
	nimbleterminal.dev/ntgamepad v0.0.0
	nimbleterminal.dev/ntpicture v0.2.0
)

require (
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20261023060817-a45b7c86333f // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/go-webgpu/goffi v0.6.8 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-runewidth v0.0.31 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/xo/terminfo v1.3.1 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
)

tool nimbleterminal.dev/booba/cmd/booba-assets

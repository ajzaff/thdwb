package mustard

import (
	"image"
	"image/draw"
	"log"
	"os"

	gg "github.com/danfragoso/thdwb/gg"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

func SetGLFWHints() {
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
}

func CreateNewWindow(title string, width int, height int, hiDPI bool) *Window {
	glw, err := glfw.CreateWindow(width, height, title, nil, nil)
	glw.SetSizeLimits(300, 200, glfw.DontCare, glfw.DontCare)

	if err != nil {
		log.Fatal(err)
	}

	xscale, yscale := float32(1), float32(1)
	if hiDPI {
		xscale, yscale = glw.GetContentScale()
	}

	window := &Window{
		title:  title,
		width:  int(float32(width) * xscale),
		height: int(float32(height) * yscale),
		hiDPI:  hiDPI,
		glw:    glw,

		defaultCursor: glfw.CreateStandardCursor(glfw.ArrowCursor),
		pointerCursor: glfw.CreateStandardCursor(glfw.HandCursor),
	}

	window.RecreateContext()
	glw.MakeContextCurrent()

	window.backend = createGLBackend()
	window.addEvents()
	window.generateTexture()
	return window
}

func (window *Window) destroy() {
	/* @TODO
	Discover what is the best way to destroy structs
	and its fields.
	*/

	window.visible = false
	window.glw.Destroy()

	window.glw = nil
	window.context = nil
	window.backend = nil
	window.frameBuffer = nil

	window.defaultCursor = nil
	window.pointerCursor = nil

	window.registeredButtons = nil
	window.registeredInputs = nil
	window.activeInput = nil

	window.rootFrame = nil

	window = nil
}

//Show - Show the window
func (window *Window) Show() {
	window.needsReflow = true
	window.visible = true
	window.glw.Show()
}

//SetRootFrame - Sets the window root frame
func (window *Window) SetRootFrame(frame *Frame) {
	window.rootFrame = frame
}

//SetRootFrame - Sets the window root frame
func (window *Window) GetSize() (int, int) {
	return window.width, window.height
}

func (window *Window) processFrame() {
	window.glw.MakeContextCurrent()
	window.glw.SwapBuffers()

	if window.needsReflow {
		drawRootFrame(window)
		window.needsReflow = false
	} else {
		redrawWidgets(window.rootFrame)
	}

	window.generateTexture()

	gl.Viewport(0, 0, int32(window.width), int32(window.height))
	gl.DrawArrays(gl.TRIANGLES, 0, 6)

	glfw.PollEvents()
}

func (window *Window) RequestReflow() {
	window.needsReflow = true
}

func (window *Window) SetTitle(title string) {
	window.glw.SetTitle(title)
}

func (window *Window) RecreateContext() {
	window.context = gg.NewContext(window.width, window.height)
}

func (window *Window) addEvents() {
	window.glw.SetFocusCallback(func(w *glfw.Window, focused bool) {
	})

	window.glw.SetSizeCallback(func(w *glfw.Window, width, height int) {
		xscale, yscale := float32(1), float32(1)
		if window.hiDPI {
			xscale, yscale = w.GetContentScale()
		}

		swidth := int(float32(width) * xscale)
		sheight := int(float32(height) * yscale)

		window.width, window.height = swidth, sheight
		window.RecreateContext()
		//window.RecreateOverlayContext()
		window.needsReflow = true
	})

	window.glw.SetCursorPosCallback(func(w *glfw.Window, x, y float64) {
		xscale, yscale := float32(1), float32(1)
		if window.hiDPI {
			xscale, yscale = w.GetContentScale()
		}

		window.cursorX, window.cursorY = x/float64(xscale), y/float64(yscale)
		window.ProcessPointerPosition()
	})

	window.glw.SetCharCallback(func(w *glfw.Window, char rune) {
		if window.activeInput != nil {
			inputVal, cursorPos := window.activeInput.value, window.activeInput.cursorPosition

			window.activeInput.value = inputVal[:len(inputVal)+cursorPos] + string(char) + inputVal[len(inputVal)+cursorPos:]
			window.activeInput.needsRepaint = true
		}
	})

	window.glw.SetCloseCallback(func(w *glfw.Window) {
		os.Exit(0)
	})

	window.glw.SetKeyCallback(func(w *glfw.Window, key glfw.Key, sc int, action glfw.Action, mods glfw.ModifierKey) {
		switch key {
		case glfw.KeyBackspace:
			if action == glfw.Repeat || action == glfw.Release {
				if window.activeInput != nil && len(window.activeInput.value) > 0 {
					if window.activeInput.cursorPosition == 0 {
						window.activeInput.value = window.activeInput.value[:len(window.activeInput.value)-1]
					} else {
						inputVal, cursorPos := window.activeInput.value, window.activeInput.cursorPosition

						if cursorPos+len(inputVal) > 0 {
							window.activeInput.value = inputVal[:len(inputVal)+cursorPos-1] + inputVal[len(inputVal)+cursorPos:]
						}
					}
					window.activeInput.needsRepaint = true
				}
			}
			break
		case glfw.KeyEscape:
			if action == glfw.Release {
				window.DestroyContextMenu()

				if window.activeInput != nil {
					window.activeInput.active = false
					window.activeInput.selected = false
					window.activeInput.needsRepaint = true
					window.activeInput = nil
				}
			}

			break
		case glfw.KeyUp:
			if action == glfw.Release || action == glfw.Repeat {
				window.ProcessArrowKeys("up")
			}
			break
		case glfw.KeyDown:
			if action == glfw.Release || action == glfw.Repeat {
				window.ProcessArrowKeys("down")
			}
			break
		case glfw.KeyLeft:
			if action == glfw.Release || action == glfw.Repeat {
				window.ProcessArrowKeys("left")
			}
			break
		case glfw.KeyRight:
			if action == glfw.Release || action == glfw.Repeat {
				window.ProcessArrowKeys("right")
			}
			break
		case glfw.KeyV:
			if action == glfw.Release && mods == glfw.ModControl {
				if window.activeInput != nil {
					if window.activeInput.cursorPosition == 0 {
						window.activeInput.value = window.activeInput.value + glfw.GetClipboardString()
					} else {
						inputVal, cursorPos := window.activeInput.value, window.activeInput.cursorPosition
						window.activeInput.value = inputVal[:len(inputVal)+cursorPos] + glfw.GetClipboardString() + inputVal[len(inputVal)+cursorPos:]
					}
					window.activeInput.needsRepaint = true
				}
			}
			break
		case glfw.KeyEnter:
			if action == glfw.Release {
				window.ProcessReturnKey()
			}
			break
		}
	})

	window.glw.SetMouseButtonCallback(func(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mod glfw.ModifierKey) {
		if action == glfw.Release {
			window.ProcessPointerClick(button)
		}
	})

	window.glw.SetScrollCallback(func(w *glfw.Window, x, y float64) {
		window.ProcessScroll(x, y)
	})
}

func compositeWidget(buffer *image.RGBA, widget Widget) {
	if widget.NeedsRepaint() {
		top, left, width, height := widget.ComputedBox().GetCoords()

		draw.Draw(buffer, image.Rectangle{
			image.Point{left, top}, image.Point{left + width, top + height},
		}, widget.Buffer(), image.Point{}, draw.Over)

		widget.SetNeedsRepaint(false)
	}
}

func compositeAll(buffer *image.RGBA, widget Widget) {
	compositeWidget(buffer, widget)

	for _, childWidget := range widget.Widgets() {
		compositeAll(buffer, childWidget)
	}
}

func (window *Window) generateTexture() {
	gl.DeleteTextures(1, &window.backend.texture)
	window.frameBuffer = window.context.Image().(*image.RGBA)

	if window.rootFrame != nil {
		compositeAll(window.frameBuffer, window.rootFrame)
	}

	if window.hasStaticOverlay {
		nBuffer := image.NewRGBA(window.frameBuffer.Bounds())
		draw.Draw(nBuffer, window.frameBuffer.Bounds(), window.frameBuffer, image.ZP, draw.Over)
		window.frameBuffer = nBuffer

		for _, overlay := range window.staticOverlays {
			draw.Draw(window.frameBuffer, overlay.buffer.Bounds().Add(overlay.position), overlay.buffer, image.ZP, draw.Over)
		}
	}

	if window.hasActiveOverlay {
		nBuffer := image.NewRGBA(window.frameBuffer.Bounds())
		draw.Draw(nBuffer, window.frameBuffer.Bounds(), window.frameBuffer, image.ZP, draw.Over)
		window.frameBuffer = nBuffer

		for _, overlay := range window.overlays {
			draw.Draw(window.frameBuffer, overlay.buffer.Bounds().Add(overlay.position), overlay.buffer, image.ZP, draw.Over)
		}
	}

	gl.GenTextures(1, &window.backend.texture)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, window.backend.texture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)

	gl.TexImage2D(
		gl.TEXTURE_2D, 0, gl.RGBA,
		int32(window.frameBuffer.Rect.Size().X), int32(window.frameBuffer.Rect.Size().Y),
		0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(window.frameBuffer.Pix),
	)

}

func (window *Window) GetCursorPosition() (float64, float64) {
	return window.cursorX, window.cursorY
}

func (window *Window) RegisterTree(tree *TreeWidget) {
	window.registeredTrees = append(window.registeredTrees, tree)
}

func (window *Window) RegisterButton(button *ButtonWidget, callback func()) {
	button.onClick = callback
	window.registeredButtons = append(window.registeredButtons, button)
}

func (window *Window) RegisterInput(input *InputWidget) {
	window.registeredInputs = append(window.registeredInputs, input)
}

func (window *Window) AttachPointerPositionEventListener(callback func(pointerX, pointerY float64)) {
	window.pointerPositionEventListeners = append(window.pointerPositionEventListeners, callback)
}

func (window *Window) AttachScrollEventListener(callback func(direction int)) {
	window.scrollEventListeners = append(window.scrollEventListeners, callback)
}

func (window *Window) AttachClickEventListener(callback func(MustardKey)) {
	window.clickEventListeners = append(window.clickEventListeners, callback)
}

func (window *Window) SetCursor(cursorType cursorType) {
	switch cursorType {
	case PointerCursor:
		window.glw.SetCursor(window.pointerCursor)
		break

	default:
		window.glw.SetCursor(window.defaultCursor)
	}
}

func (window *Window) AddOverlay(overlay *Overlay) {
	window.overlays = append(
		window.overlays,
		overlay,
	)

	window.hasActiveOverlay = true
}

func (window *Window) RemoveOverlay(overlay *Overlay) {
	for idx, cOverlay := range window.overlays {
		if cOverlay == overlay {
			window.overlays = append(window.overlays[:idx], window.overlays[idx+1:]...)
			break
		}
	}

	if len(window.overlays) < 1 {
		window.hasActiveOverlay = false
	}
}

func (window *Window) AddStaticOverlay(overlay *Overlay) {
	window.staticOverlays = append(
		window.staticOverlays,
		overlay,
	)

	window.hasStaticOverlay = true
}

func (window *Window) RemoveStaticOverlay(ref string) {
	for idx, cOverlay := range window.staticOverlays {
		if cOverlay.ref == ref {
			window.staticOverlays = append(window.staticOverlays[:idx], window.staticOverlays[idx+1:]...)
		}
	}

	if len(window.staticOverlays) < 1 {
		window.hasStaticOverlay = false
	}
}

func CreateStaticOverlay(ref string, ctx *gg.Context, position image.Point) *Overlay {
	buffer := ctx.Image().(*image.RGBA)

	return &Overlay{
		ref:    ref,
		active: true,

		top:  float64(position.Y),
		left: float64(position.X),

		width:  float64(buffer.Rect.Max.X),
		height: float64(buffer.Rect.Max.Y),

		position: position,
		buffer:   buffer,
	}
}

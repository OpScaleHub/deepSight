package main

/*
#cgo pkg-config: gimp-3.0 gimpui-3.0
#include <libgimp/gimp.h>
#include <libgimp/gimppdb.h>
#include <libgimp/gimpui.h>
#include <string.h> // For C.CString, C.GoStringN
#include <stdlib.h> // For C.free
#include <glib.h>   // For C types like gint32

// Forward declarations for Go functions exported to C
void query(void);
void run(const gchar *name, gint nparams, const GimpParam *param, gint *nreturn_vals, GimpParam **return_vals);

// C helper function to create a string for GIMP
static gchar* to_gchar(const char* s) {
    return (gchar*)s;
}

// C helper to get a widget from a GimpDialog
static GtkWidget* get_widget_from_dialog(GimpDialog *dialog, const gchar *role) {
    return gimp_dialog_get_widget(dialog, role);
}

*/
import "C"

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"log"
	"os"
	"strings"
	"unsafe"

	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

const (
	pluginName        = "gimini-generate"
	pluginBlurb       = "Gemini AI Image Generation"
	pluginDescription = "Uses Google's Gemini API to generate or edit images."
	pluginAuthor      = "Gemini Code Assist"
	pluginCopyright   = "OpScaleHub"
	pluginDate        = "2025"
	pluginMenuLabel   = "Gemini Image Generator..."
	pluginMenuPath    = "<Image>/Filters/AI Tools"
	pluginConfigKey   = "gimini-api-key"
)

// Plugin constants
const (
	MODE_TEXT_TO_IMAGE = iota
	MODE_IMAGE_TO_IMAGE
)

// Global struct to hold UI state and GIMP context
var pluginVals struct {
	promptText string
	apiKey     string
	mode       int
	run        bool // Tracks if the user clicked "OK"

	// GIMP context
	imageID    C.gint32
	drawableID C.gint32
}

// main is required for a Go binary, but for a GIMP plugin,
// the C functions `query` and `run` are the entry points.
func main() {}

//export query
func query() {
	// C strings for GIMP registration
	cPluginName := C.CString(pluginName)
	defer C.free(unsafe.Pointer(cPluginName))
	cPluginBlurb := C.CString(pluginBlurb)
	defer C.free(unsafe.Pointer(cPluginBlurb))
	cPluginDescription := C.CString(pluginDescription)
	defer C.free(unsafe.Pointer(cPluginDescription))
	cPluginAuthor := C.CString(pluginAuthor)
	defer C.free(unsafe.Pointer(cPluginAuthor))
	cPluginCopyright := C.CString(pluginCopyright)
	defer C.free(unsafe.Pointer(cPluginCopyright))
	cPluginDate := C.CString(pluginDate)
	defer C.free(unsafe.Pointer(cPluginDate))
	cMenuLabel := C.CString(pluginMenuLabel)
	defer C.free(unsafe.Pointer(cMenuLabel))
	cMenuPath := C.CString(pluginMenuPath)
	defer C.free(unsafe.Pointer(cMenuPath))

	// Define the arguments for the PDB procedure
	// GIMP passes these to the `run` function
	args := C.GimpParamDef{
		C.GIMP_PDB_INT32,
		C.CString("run-mode"),
		C.CString("Run mode"),
	}

	// Register the plugin procedure with GIMP
	C.gimp_install_procedure(
		cPluginName,
		cPluginBlurb,
		cPluginDescription,
		cPluginAuthor,
		cPluginCopyright,
		cPluginDate,
		cMenuLabel,
		C.to_gchar(C.CString("RGB*, GRAY*")), // Image types
		C.GIMP_PLUGIN,
		1, 0, // nparams, nreturn_vals for the procedure itself
		&args, nil,
	)

	// Associate the menu path with the procedure
	C.gimp_plugin_menu_register(cPluginName, cMenuPath)
}

//export run
func run(name *C.gchar, nparams C.gint, param *C.GimpParam, nreturn_vals *C.gint, return_vals **C.GimpParam) {
	// Setup return values for GIMP
	*nreturn_vals = 1
	*return_vals = C.gimp_param_new(0)
	status := C.GIMP_PDB_SUCCESS

	// Get image and drawable IDs from GIMP
	pluginVals.imageID = param.data.d_image
	pluginVals.drawableID = param.data.d_drawable

	// Initialize GIMP UI
	C.gimp_ui_init(C.CString(pluginName), C.gboolean(0))

	// Create the UI dialog and check if the user clicked "OK"
	if !createDialog() {
		status = C.GIMP_PDB_CANCEL
		(*return_vals).data.d_status = status
		return
	}

	// Show a progress bar
	C.gimp_progress_init_printf(C.CString("Contacting Gemini API..."))

	// Execute the main logic
	err := runPluginLogic()
	if err != nil {
		status = C.GIMP_PDB_EXECUTION_ERROR
		// Show an error dialog to the user
		errorMsg := C.CString(fmt.Sprintf("GIMini Error: %v", err))
		defer C.free(unsafe.Pointer(errorMsg))
		C.gimp_message(errorMsg)
	}

	// End progress bar
	C.gimp_progress_end()

	// Set final status and return
	(*return_vals).data.d_status = status
}

// createDialog builds and runs the GTK dialog for the plugin.
// It returns true if the user clicks "OK", false otherwise.
func createDialog() bool {
	pluginVals.run = false // Reset run state

	// Create dialog
	cDialogTitle := C.CString(pluginMenuLabel)
	defer C.free(unsafe.Pointer(cDialogTitle))
	dialog := C.gimp_dialog_new(cDialogTitle, C.CString(pluginName), nil, 0, nil)

	// Main content area
	contentArea := C.gtk_dialog_get_content_area(GTK_DIALOG(unsafe.Pointer(dialog)))

	// --- API Key Entry ---
	apiKeyFrame := C.gtk_frame_new(C.CString("Gemini API Key"))
	C.gtk_box_pack_start(GTK_BOX(unsafe.Pointer(contentArea)), apiKeyFrame, C.gboolean(1), C.gboolean(1), 0)
	apiKeyEntry := C.gtk_entry_new()
	C.gtk_container_add(GTK_CONTAINER(unsafe.Pointer(apiKeyFrame)), apiKeyEntry)
	// Load saved API key
	savedKey := loadAPIKey()
	if savedKey != "" {
		C.gtk_entry_set_text(GTK_ENTRY(unsafe.Pointer(apiKeyEntry)), C.CString(savedKey))
	}

	// --- Prompt Entry ---
	promptFrame := C.gtk_frame_new(C.CString("Prompt"))
	C.gtk_box_pack_start(GTK_BOX(unsafe.Pointer(contentArea)), promptFrame, C.gboolean(1), C.gboolean(1), 0)
	scrolledWindow := C.gtk_scrolled_window_new(nil, nil)
	C.gtk_container_add(GTK_CONTAINER(unsafe.Pointer(promptFrame)), scrolledWindow)
	promptView := C.gtk_text_view_new()
	C.gtk_text_view_set_wrap_mode(GTK_TEXT_VIEW(unsafe.Pointer(promptView)), C.GTK_WRAP_WORD_CHAR)
	C.gtk_scrolled_window_add_with_viewport(GTK_SCROLLED_WINDOW(unsafe.Pointer(scrolledWindow)), promptView)
	C.gtk_widget_set_size_request(scrolledWindow, -1, 150)

	// --- Mode Selection ---
	modeFrame := C.gtk_frame_new(C.CString("Mode"))
	C.gtk_box_pack_start(GTK_BOX(unsafe.Pointer(contentArea)), modeFrame, C.gboolean(1), C.gboolean(1), 0)
	modeBox := C.gtk_box_new(C.GTK_ORIENTATION_VERTICAL, 6)
	C.gtk_container_add(GTK_CONTAINER(unsafe.Pointer(modeFrame)), modeBox)

	radioGen := C.gtk_radio_button_new_with_label(nil, C.CString("Generate New Image (Text-to-Image)"))
	C.gtk_box_pack_start(GTK_BOX(unsafe.Pointer(modeBox)), radioGen, C.gboolean(1), C.gboolean(1), 0)
	radioEdit := C.gtk_radio_button_new_with_label_from_widget(GTK_RADIO_BUTTON(unsafe.Pointer(radioGen)), C.CString("Edit Current Layer (Image-to-Image)"))
	C.gtk_box_pack_start(GTK_BOX(unsafe.Pointer(modeBox)), radioEdit, C.gboolean(1), C.gboolean(1), 0)
	C.gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(unsafe.Pointer(radioGen)), C.gboolean(1)) // Default to Generate

	// Run the dialog
	C.gtk_window_present(GTK_WINDOW(unsafe.Pointer(dialog)))
	response := C.gimp_dialog_run(dialog)

	if response == C.GTK_RESPONSE_OK {
		pluginVals.run = true

		// Get API Key
		cKey := C.gtk_entry_get_text(GTK_ENTRY(unsafe.Pointer(apiKeyEntry)))
		pluginVals.apiKey = C.GoString(cKey)

		// Get Prompt
		buffer := C.gtk_text_view_get_buffer(GTK_TEXT_VIEW(unsafe.Pointer(promptView)))
		var start, end C.GtkTextIter
		C.gtk_text_buffer_get_start_iter(buffer, &start)
		C.gtk_text_buffer_get_end_iter(buffer, &end)
		cPrompt := C.gtk_text_buffer_get_text(buffer, &start, &end, C.gboolean(0))
		defer C.g_free(C.gpointer(cPrompt))
		pluginVals.promptText = C.GoString(cPrompt)

		// Get Mode
		if C.gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(unsafe.Pointer(radioGen))) == C.gboolean(1) {
			pluginVals.mode = MODE_TEXT_TO_IMAGE
		} else {
			pluginVals.mode = MODE_IMAGE_TO_IMAGE
		}
	}

	C.gtk_widget_destroy(GTK_WIDGET(unsafe.Pointer(dialog)))
	return pluginVals.run
}

// runPluginLogic is the main business logic handler.
func runPluginLogic() error {
	if pluginVals.promptText == "" {
		return fmt.Errorf("prompt cannot be empty")
	}
	if pluginVals.apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// Save the API key for the next session
	saveAPIKey(pluginVals.apiKey)

	log.Printf("GIMini: Mode=%d, Prompt='%s'", pluginVals.mode, pluginVals.promptText)

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(pluginVals.apiKey))
	if err != nil {
		return fmt.Errorf("failed to create genai client: %w", err)
	}
	defer client.Close()

	var generatedImageBytes []byte

	switch pluginVals.mode {
	case MODE_TEXT_TO_IMAGE:
		// TODO: Replace with a real image generation model when available.
		// Using a text model to simulate the flow.
		model := client.GenerativeModel("gemini-pro")
		resp, err := model.GenerateContent(ctx, genai.Text(fmt.Sprintf("A placeholder for an image of: %s", pluginVals.promptText)))
		if err != nil {
			return fmt.Errorf("gemini API call failed: %w", err)
		}
		// This is a placeholder. A real image model would return image bytes.
		// For now, we'll create a dummy black image.
		log.Printf("GIMini: API response received (simulated).")
		generatedImageBytes, err = createDummyImage()
		if err != nil {
			return fmt.Errorf("failed to create dummy image: %w", err)
		}
		_ = resp // Avoid unused variable error

	case MODE_IMAGE_TO_IMAGE:
		// This is a placeholder for the Image-to-Image logic.
		log.Println("GIMini: Starting Image-to-Image mode.")

		// 1. Get active layer as PNG bytes
		C.gimp_progress_set_text(C.CString("Reading layer data..."))
		inputImageBytes, err := getLayerAsPNG(pluginVals.drawableID)
		if err != nil {
			return fmt.Errorf("could not read layer data: %w", err)
		}

		// 2. Send to Gemini Vision model
		C.gimp_progress_set_text(C.CString("Sending data to Gemini Vision API..."))
		model := client.GenerativeModel("gemini-pro-vision")
		prompt := []genai.Part{
			genai.ImageData("png", inputImageBytes),
			genai.Text(pluginVals.promptText),
		}

		resp, err := model.GenerateContent(ctx, prompt...)
		if err != nil {
			return fmt.Errorf("gemini vision API call failed: %w", err)
		}

		// 3. Process response
		// NOTE: The Vision model currently returns text. A future image-generation
		// model would return image data. For now, we log the text response and
		// generate a placeholder image to prove the pipeline works.
		log.Printf("GIMini: Vision API response received.")
		if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
			log.Printf("GIMini: Gemini Response: '%s'", resp.Candidates[0].Content.Parts[0])
		}

		generatedImageBytes, err = createDummyImage() // Placeholder for actual image data
		if err != nil {
			return fmt.Errorf("failed to create placeholder image: %w", err)
		}

	default:
		return fmt.Errorf("unknown mode selected")
	}

	if generatedImageBytes != nil {
		return createLayerFromBytes(generatedImageBytes)
	}

	return nil
}

// createLayerFromBytes decodes image data and creates a new layer in GIMP.
func createLayerFromBytes(data []byte) error {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to decode image data from API: %w", err)
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Create a new layer in GIMP
	layerName := getLayerName(pluginVals.promptText)
	cLayerName := C.CString(layerName)
	defer C.free(unsafe.Pointer(cLayerName))

	// Assuming RGBA for now. A real implementation needs to handle different image types.
	newLayerID := C.gimp_layer_new(
		pluginVals.imageID,
		cLayerName,
		C.gint(width),
		C.gint(height),
		C.GIMP_RGBA_IMAGE, // Type
		100,               // Opacity
		C.GIMP_NORMAL_MODE,
	)

	// Add the new layer to the image
	C.gimp_image_insert_layer(pluginVals.imageID, newLayerID, nil, -1)

	// Get a pixel region to write our data into
	shadow := C.gimp_drawable_get_shadow_buffer(newLayerID)
	destRect := C.GimpRectangle{0, 0, C.gint(width), C.gint(height)}
	C.gimp_pixel_rgn_init(shadow, C.gimp_drawable_get_buffer(newLayerID), 0, 0, C.gint(width), C.gint(height), C.TRUE, C.TRUE) // shadow_is_writeable, shadow_is_dirty

	// Prepare pixel data for GIMP (non-premultiplied RGBA)
	bpp := 4 // bytes per pixel (RGBA)
	stride := width * bpp
	pixels := make([]byte, stride*height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := y*stride + x*bpp
			r, g, b, a := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			// Convert from premultiplied Alpha to non-premultiplied
			pixels[offset+0] = byte(r >> 8)
			pixels[offset+1] = byte(g >> 8)
			pixels[offset+2] = byte(b >> 8)
			pixels[offset+3] = byte(a >> 8)
		}
	}

	// Write the pixel data to the GIMP layer
	C.gimp_pixel_rgn_set_rect(shadow, (*C.guint8)(unsafe.Pointer(&pixels[0])), destRect.x, destRect.y, destRect.width, destRect.height)

	// Update the drawable
	C.gimp_drawable_flush(newLayerID)
	C.gimp_drawable_merge_shadow(newLayerID, C.TRUE)
	C.gimp_drawable_update(newLayerID, destRect.x, destRect.y, destRect.width, destRect.height)

	return nil
}

// getLayerAsPNG reads the pixel data from a GIMP drawable and encodes it as a PNG.
func getLayerAsPNG(drawableID C.gint32) ([]byte, error) {
	// Get drawable properties
	width := C.gimp_drawable_width(drawableID)
	height := C.gimp_drawable_height(drawableID)
	bpp := C.gimp_drawable_bpp(drawableID)

	// Get a pixel region to read from
	rgn := C.gimp_pixel_rgn_new(C.gimp_drawable_get_buffer(drawableID), 0, 0, width, height, C.FALSE, C.FALSE)
	if rgn == nil {
		return nil, fmt.Errorf("failed to create pixel region for drawable %d", drawableID)
	}

	// Prepare a Go slice to hold the pixel data
	stride := int(width) * int(bpp)
	pixels := make([]byte, stride*int(height))

	// Read the rectangle of pixels from GIMP into our Go slice
	C.gimp_pixel_rgn_get_rect(rgn, (*C.guint8)(unsafe.Pointer(&pixels[0])), 0, 0, width, height)

	// Create a Go image.Image from the raw pixel data.
	// GIMP provides non-premultiplied RGBA data, which matches image.RGBA.
	var img image.Image
	switch bpp {
	case 4: // RGBA
		img = &image.RGBA{
			Pix:    pixels,
			Stride: stride,
			Rect:   image.Rect(0, 0, int(width), int(height)),
		}
	case 3: // RGB
		img = &image.NRGBA{ // Use NRGBA and manually copy to handle 3-byte stride
			Pix:    make([]byte, int(width)*int(height)*4),
			Stride: int(width) * 4,
			Rect:   image.Rect(0, 0, int(width), int(height)),
		}
		// Manually copy RGB to RGBA with full alpha
		for y := 0; y < int(height); y++ {
			for x := 0; x < int(width); x++ {
				srcOff := y*stride + x*3
				dstOff := y*img.(*image.NRGBA).Stride + x*4
				copy(img.(*image.NRGBA).Pix[dstOff:dstOff+3], pixels[srcOff:srcOff+3])
				img.(*image.NRGBA).Pix[dstOff+3] = 255 // Opaque Alpha
			}
		}
	default:
		return nil, fmt.Errorf("unsupported bytes-per-pixel: %d", bpp)
	}

	// Encode the image.Image to a PNG byte buffer
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode layer to PNG: %w", err)
	}

	log.Printf("GIMini: Successfully encoded layer (%dx%d, %d bpp) to PNG.", width, height, bpp)
	return buf.Bytes(), nil
}

// --- Helper Functions ---

func saveAPIKey(key string) {
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	cConfigKey := C.CString(pluginConfigKey)
	defer C.free(unsafe.Pointer(cConfigKey))

	// GIMP 3 uses parasites to store persistent plugin data with an image
	C.gimp_parasite_set(cConfigKey, C.GIMP_PARASITE_PERSISTENT, C.gint(len(key)), C.gpointer(cKey))
}

func loadAPIKey() string {
	cConfigKey := C.CString(pluginConfigKey)
	defer C.free(unsafe.Pointer(cConfigKey))

	parasite := C.gimp_parasite_get(cConfigKey)
	if parasite == nil {
		return ""
	}
	defer C.gimp_parasite_free(parasite)

	return C.GoStringN((*C.char)(C.gimp_parasite_data(parasite)), C.int(C.gimp_parasite_data_size(parasite)))
}

func getLayerName(prompt string) string {
	words := strings.Fields(prompt)
	maxWords := 5
	if len(words) < maxWords {
		maxWords = len(words)
	}
	shortPrompt := strings.Join(words[:maxWords], " ")
	return fmt.Sprintf("Gemini Gen: %s...", shortPrompt)
}

// createDummyImage creates a 512x512 black PNG for placeholder use.
func createDummyImage() ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 512, 512))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Cgo requires some type casting that is verbose. These helpers make it cleaner.
func GTK_BOX(p unsafe.Pointer) *C.GtkBox                        { return (*C.GtkBox)(p) }
func GTK_FRAME(p unsafe.Pointer) *C.GtkFrame                    { return (*C.GtkFrame)(p) }
func GTK_ENTRY(p unsafe.Pointer) *C.GtkEntry                    { return (*C.GtkEntry)(p) }
func GTK_TEXT_VIEW(p unsafe.Pointer) *C.GtkTextView             { return (*C.GtkTextView)(p) }
func GTK_SCROLLED_WINDOW(p unsafe.Pointer) *C.GtkScrolledWindow { return (*C.GtkScrolledWindow)(p) }
func GTK_CHECK_BUTTON(p unsafe.Pointer) *C.GtkCheckButton       { return (*C.GtkCheckButton)(p) }
func GTK_WINDOW(p unsafe.Pointer) *C.GtkWindow                  { return (*C.GtkWindow)(p) }
func GTK_DIALOG(p unsafe.Pointer) *C.GtkDialog                  { return (*C.GtkDialog)(p) }
func GTK_WIDGET(p unsafe.Pointer) *C.GtkWidget                  { return (*C.GtkWidget)(p) }
func GTK_CONTAINER(p unsafe.Pointer) *C.GtkContainer            { return (*C.GtkContainer)(p) }
func GTK_TOGGLE_BUTTON(p unsafe.Pointer) *C.GtkToggleButton     { return (*C.GtkToggleButton)(p) }
func GTK_RADIO_BUTTON(p unsafe.Pointer) *C.GtkRadioButton       { return (*C.GtkRadioButton)(p) }

func init() {
	// Configure logging to a file for debugging GIMP plugins
	f, err := os.OpenFile("gimini-debug.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(f)
	log.Println("GIMini plugin initialized")
}

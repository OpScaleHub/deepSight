# Project Specification: Gemini GIMP Plugin (GIMini)

**Project Name:** `Gemini GIMP Plugin` (GIMini)
**Document Version:** 1.0
**Date:** October 15, 2025
**Target Platform:** GIMP 3.x (using GTK3 API)
**Development Language:** Golang (via Cgo bindings)
**Repository:** [https://github.com/OpScaleHub/GIMini.git](https://github.com/OpScaleHub/GIMini.git)

---

## 1. Project Goal

The primary goal is to create a robust, high-performance GIMP plugin written in Go (Golang) that leverages the Google Gemini API to enable powerful AI image generation and editing capabilities directly within the GIMP desktop environment.

This is an open-source project. The plugin will be distributed as a single, self-contained, compiled binary for ease of installation.

## 2. Core Features and User Experience

The plugin must expose the following procedures to the GIMP Procedure Database (PDB) and feature a clean, responsive UI dialog.

### 2.1 UI/Interface

*   **Location:** Access via a new menu entry: `Filters -> AI Tools -> Gemini Image Generator`.
*   **API Key Management:** A persistent input field for the user to enter their Gemini API key. This key must be securely stored using GIMP's configuration mechanism for persistence across sessions.
*   **Prompt Entry:** A multi-line text area for the user to enter detailed text prompts.
*   **Mode Selection:** A radio button or dropdown to select the operation mode:
    *   Generate New Image (Text-to-Image)
    *   Edit Current Layer (Image-to-Image/Inpainting)

### 2.2 Core Functionality

| Feature              | Description                                                                                                                                      | Gemini API Requirement                                                                              |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------- |
| **Text-to-Image**    | Creates a new image based on the text prompt and inserts the result as a new layer in the active GIMP document.                                    | Call to an image generation model (e.g., `imagen-3.0-generate-002:predict`).                        |
| **Image-to-Image Edit** | Takes the active GIMP layer (or selection) as an input image and modifies it based on the prompt. The result is added as a new layer.           | Call to an image editing model (e.g., `gemini-2.5-flash-image-preview:generateContent` with image input). |
| **Selection/Masking**  | If a selection is active in GIMP, the plugin must use this selection to determine the area of the image to be edited or replaced (for inpainting). | The plugin must extract the selected area and its mask data and encode them for the API payload.      |
| **Output Layer Naming** | New layers created by the plugin must be descriptively named (e.g., "Gemini Gen: [First few words of prompt]").                                   | N/A (Internal GIMP operation).                                                                      |

## 3. Technical Architecture and Requirements

### 3.1 Language and Interoperability

*   **Primary Language:** Go (Golang).
*   **GIMP Interface:** Cgo is mandatory for interfacing with GIMP's C libraries (`libgimp`). All PDB registration and UI/image manipulation must be performed through Cgo bindings to the GIMP 3.0 API.

### 3.2 Data Flow and Processing

The plugin must implement the following pipeline for Image-to-Image editing:

1.  **Capture GIMP Data:** The plugin calls GIMP API functions (via Cgo) to retrieve the active layer's pixel data and any active selection mask data.
2.  **Data Encoding (Go):** The raw pixel data (and mask data, if applicable) must be processed in the Go runtime and encoded into a standard format like Base64 PNG or JPEG (PNG is preferred for mask handling).
3.  **API Call (Go):** The encoded image/mask data, the user prompt, and the API key are assembled into a JSON payload and sent to the Gemini API endpoint using the official Google GenAI Go SDK or standard `net/http`.
4.  **Response Handling (Go):** The base64-encoded image result is received from the API and decoded back into raw pixel data in Go.
5.  **Create GIMP Layer:** The Go runtime uses Cgo to call GIMP API functions to create a new GIMP layer and populate it with the processed pixel data, inserting it into the active image stack.

### 3.3 Dependencies

*   **GIMP 3.0 Headers/Libraries:** Required for compilation and linking via Cgo.
*   **Go Standard Library:** For HTTP/JSON handling and base64 encoding/decoding.
*   **Official Go SDK for Gemini/GenAI:** Recommended to simplify API interactions.
*   **Go Image Libraries:** For robust image format conversions (e.g., `image/png`, `image/jpeg`).

### 3.4 Authentication Protocol

The plugin will manage the API key as follows:

*   When the user enters the key, the plugin saves it using GIMP's configuration system (e.g., `gimp_config_set_value`).
*   The key is loaded when the plugin initializes.
*   All calls to the Gemini API must include the API key as required by the API. The key must **never** be logged to the console or stored in source code.

## 4. Deliverables and Success Criteria

### 4.1 Deliverables

*   **`main.go`**: A well-commented Go source file containing all the necessary Cgo directives, GIMP function wrappers, business logic, and API communication logic.
*   **`GIMini` (Binary)**: The final compiled binary file ready to be placed in the GIMP plugins folder.
*   **Build Instructions**: A `Makefile` or script to simplify the compilation process.

### 4.2 Success Criteria

*   The plugin compiles successfully on a standard Linux/macOS/Windows environment.
*   The plugin is recognized by GIMP and appears in the `Filters` menu.
*   The UI dialog opens correctly and allows the user to input the prompt and API key.
*   **Test 1 (Text-to-Image):** A new layer is generated accurately based on a text prompt.
*   **Test 2 (Image-to-Image):** An existing layer is correctly modified using a text prompt, and the result is placed on a new layer.
*   **Test 3 (Error Handling):** Invalid API keys or failed network calls result in a user-friendly message box within GIMP (not a console crash).

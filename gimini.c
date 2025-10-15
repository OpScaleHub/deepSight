#include <libgimp/gimp.h>
#include <gtk/gtk.h>

// This is a C shim to bridge Go and the GIMP/GTK libraries.

void gimini_gimp_ui_init(const gchar *name, gboolean a) {
    gimp_ui_init(name, a);
}
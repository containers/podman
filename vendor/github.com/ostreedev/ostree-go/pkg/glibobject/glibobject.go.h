#include <glib.h>

static char *
_g_error_get_message (GError *error)
{
  g_assert (error != NULL);
  return error->message;
}

static const char *
_g_variant_lookup_string (GVariant *v, const char *key)
{
  const char *r;
  if (g_variant_lookup (v, key, "&s", &r))
    return r;
  return NULL;
}

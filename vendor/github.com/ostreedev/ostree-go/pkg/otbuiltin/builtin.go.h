#ifndef BUILTIN_GO_H
#define BUILTIN_GO_H

#include <glib.h>
#include <ostree.h>
#include <string.h>
#include <fcntl.h>

static guint32 owner_uid;
static guint32 owner_gid;

static void
_ostree_repo_append_modifier_flags(OstreeRepoCommitModifierFlags *flags, int flag) {
  *flags |= flag;
}

struct CommitFilterData {
  GHashTable *mode_adds;
  GHashTable *skip_list;
};

typedef struct CommitFilterData CommitFilterData;

static char* _gptr_to_str(gpointer p)
{
    return (char*)p;
}

// The following 3 functions are wrapper functions for macros since CGO can't parse macros
static OstreeRepoFile*
_ostree_repo_file(GFile *file)
{
  return OSTREE_REPO_FILE (file);
}

static gpointer
_guint_to_pointer (guint u)
{
  return GUINT_TO_POINTER (u);
}

static const GVariantType*
_g_variant_type (char *type)
{
  return G_VARIANT_TYPE (type);
}

static int
_at_fdcwd ()
{
  return AT_FDCWD;
}

static guint64
_guint64_from_be (guint64 val)
{
  return GUINT64_FROM_BE (val);
}



// These functions are wrappers for variadic functions since CGO can't parse variadic functions
static void
_g_printerr_onearg (char* msg,
                    char* arg)
{
  g_printerr("%s %s\n", msg, arg);
}

static void
_g_set_error_onearg (GError *err,
                     char*  msg,
                     char*  arg)
{
  g_set_error(&err, G_IO_ERROR, G_IO_ERROR_FAILED, "%s %s", msg, arg);
}

static void
_g_variant_builder_add_twoargs (GVariantBuilder*     builder,
                                const char    *format_string,
                                char          *arg1,
                                GVariant      *arg2)
{
  g_variant_builder_add(builder, format_string, arg1, arg2);
}

static GHashTable*
_g_hash_table_new_full ()
{
  return g_hash_table_new_full(g_str_hash, g_str_equal, g_free, NULL);
}

static void
_g_variant_get_commit_dump (GVariant    *variant,
                            const char  *format,
                            char        **subject,
                            char        **body,
                            guint64     *timestamp)
{
  return g_variant_get (variant, format, NULL, NULL, NULL, subject, body, timestamp, NULL, NULL);
}

static guint32
_binary_or (guint32 a, guint32 b)
{
  return a | b;
}

static void
_cleanup (OstreeRepo                *self,
          OstreeRepoCommitModifier  *modifier,
          GCancellable              *cancellable,
          GError                    **out_error)
{
  if (self)
    ostree_repo_abort_transaction(self, cancellable, out_error);
  if (modifier)
    ostree_repo_commit_modifier_unref (modifier);
}

// The following functions make up a commit_filter function that gets passed into
// another C function (and thus can't be a go function) as well as its helpers
static OstreeRepoCommitFilterResult
_commit_filter (OstreeRepo         *self,
               const char         *path,
               GFileInfo          *file_info,
               gpointer            user_data)
{
  struct CommitFilterData *data = user_data;
  GHashTable *mode_adds = data->mode_adds;
  GHashTable *skip_list = data->skip_list;
  gpointer value;

  if (owner_uid >= 0)
    g_file_info_set_attribute_uint32 (file_info, "unix::uid", owner_uid);
  if (owner_gid >= 0)
    g_file_info_set_attribute_uint32 (file_info, "unix::gid", owner_gid);

  if (mode_adds && g_hash_table_lookup_extended (mode_adds, path, NULL, &value))
    {
      guint current_mode = g_file_info_get_attribute_uint32 (file_info, "unix::mode");
      guint mode_add = GPOINTER_TO_UINT (value);
      g_file_info_set_attribute_uint32 (file_info, "unix::mode",
                                        current_mode | mode_add);
      g_hash_table_remove (mode_adds, path);
    }

  if (skip_list && g_hash_table_contains (skip_list, path))
    {
      g_hash_table_remove (skip_list, path);
      return OSTREE_REPO_COMMIT_FILTER_SKIP;
    }

  return OSTREE_REPO_COMMIT_FILTER_ALLOW;
}


static void
_set_owner_uid (guint32 uid)
{
  owner_uid = uid;
}

static void _set_owner_gid (guint32 gid)
{
  owner_gid = gid;
}

// Wrapper function for a function that takes a C function as a parameter.
// That translation doesn't work in go
static OstreeRepoCommitModifier*
_ostree_repo_commit_modifier_new_wrapper (OstreeRepoCommitModifierFlags  flags,
                                          gpointer                       user_data,
                                          GDestroyNotify                 destroy_notify)
{
  return ostree_repo_commit_modifier_new(flags, _commit_filter, user_data, destroy_notify);
}

#endif

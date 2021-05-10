# Configuration file for the Sphinx documentation builder.
#
# This file only contains a selection of the most common options. For a full
# list see the documentation:
# https://www.sphinx-doc.org/en/master/usage/configuration.html

# -- Path setup --------------------------------------------------------------

# If extensions (or modules to document with autodoc) are in another directory,
# add these directories to sys.path here. If the directory is relative to the
# documentation root, use os.path.abspath to make it absolute, like shown here.
#
# import os
# import sys
# sys.path.insert(0, os.path.abspath('.'))

import re
from recommonmark.transform import AutoStructify

# -- Project information -----------------------------------------------------

project = "Podman"
copyright = "2019, team"
author = "team"


# -- General configuration ---------------------------------------------------

# Add any Sphinx extension module names here, as strings. They can be
# extensions coming with Sphinx (named 'sphinx.ext.*') or your custom
# ones.
extensions = ["sphinx_markdown_tables", "recommonmark"]

# Add any paths that contain templates here, relative to this directory.
templates_path = ["_templates"]

# List of patterns, relative to source directory, that match files and
# directories to ignore when looking for source files.
# This pattern also affects html_static_path and html_extra_path.
exclude_patterns = []

master_doc = "index"

# Configure smartquotes to only transform quotes and ellipses, not dashes
smartquotes_action = "qe"


# -- Options for HTML output -------------------------------------------------

# The theme to use for HTML and HTML Help pages.  See the documentation for
# a list of builtin themes.
#
html_theme = "alabaster"

# Add any paths that contain custom static files (such as style sheets) here,
# relative to this directory. They are copied after the builtin static files,
# so a file named "default.css" will overwrite the builtin "default.css".
html_static_path = ["_static"]

html_css_files = [
    "custom.css",
]

# -- Extension configuration -------------------------------------------------


def convert_markdown_title(app, docname, source):
    # Process markdown files only
    docpath = app.env.doc2path(docname)
    if docpath.endswith(".md"):
        # Convert pandoc title line into eval_rst block for recommonmark
        source[0] = re.sub(r"^% (.*)", r"```eval_rst\n.. title:: \g<1>\n```", source[0])


def setup(app):
    app.connect("source-read", convert_markdown_title)

    app.add_config_value(
        "recommonmark_config",
        {
            "enable_eval_rst": True,
            "enable_auto_doc_ref": False,
            "enable_auto_toc_tree": False,
            "enable_math": False,
            "enable_inline_math": False,
        },
        True,
    )
    app.add_transform(AutoStructify)

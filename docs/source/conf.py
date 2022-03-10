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

# -- Project information -----------------------------------------------------

project = "Podman"
copyright = "2019, team"
author = "team"


# -- General configuration ---------------------------------------------------

# Add any Sphinx extension module names here, as strings. They can be
# extensions coming with Sphinx (named 'sphinx.ext.*') or your custom
# ones.
extensions = ["myst_parser"]

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

# IMPORTANT: explicitly unset the extensions, by default dollarmath is enabled.
# We use the dollar sign as text and do not want it to be interpreted as math expression.
myst_enable_extensions = []


def convert_markdown_title(app, docname, source):
    # Process markdown files only
    docpath = app.env.doc2path(docname)
    if docpath.endswith(".md"):
        # Convert pandoc title line into eval_rst block for myst_parser
        #
        # Remove the ending "(1)" to avoid it from being displayed
        # in the web tab. Often such a text indicates that
        # a web page got an update. For instance GitHub issues
        # shows the number of new comments that have been written
        # after the user's last visit.
        source[0] = re.sub(r"^% (.*)(\(\d\))", r"```{title} \g<1>\n```", source[0])

def setup(app):
    app.connect("source-read", convert_markdown_title)

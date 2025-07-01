# Podman Certificate Generator

A web-based tool to generate personalized certificates for first-time Podman contributors.

## Overview

This certificate generator creates beautiful, printable certificates to celebrate and recognize first-time contributors to the Podman project. It helps build community engagement and provides a memorable way to acknowledge new contributors' efforts.

## Features

- **Interactive Web Interface**: Easy-to-use form for generating certificates
- **Real-time Preview**: See the certificate as you customize it
- **Professional Design**: Clean, modern certificate layout with Podman branding
- **Downloadable Output**: Generate HTML certificates that can be printed or saved as PDF
- **Customizable**: Support for contributor name, PR number, and date

## Files

- `certificate_generator.html` - Main interactive certificate generator interface
- `certificate_template.html` - Base certificate template (standalone version)
- `1stpr.png` - Sample certificate image showing the design concept
- `README.md` - This documentation file

## Usage

### Using the Generator

1. Open `certificate_generator.html` in a web browser
2. Fill in the form fields:
   - **Contributor Name**: The name to appear on the certificate
   - **PR Number**: The pull request number for their first contribution
   - **Date**: The date of the contribution (defaults to today)
3. Preview the certificate in real-time as you type
4. Click "Download Certificate" to save the personalized certificate as an HTML file

### Using the Template Directly

The `certificate_template.html` file can be used as a standalone template if you prefer to manually edit the HTML:

1. Open `certificate_template.html` in a text editor
2. Replace the placeholder text:
   - `[Contributor Name]` with the actual contributor name
   - `[PR Number]` with the pull request number
   - `[Date]` with the contribution date
3. Save and open in a browser to view/print

## Certificate Design

The certificates feature:
- Podman purple branding (#892ca0)
- Professional typography and layout
- Seal emoji (🦭) with graduation cap decoration
- Clean, printable design suitable for framing
- Responsive design that works on different screen sizes

## Technical Details

- Pure HTML/CSS/JavaScript implementation
- No external dependencies
- Works in all modern browsers
- Print-friendly CSS styles
- Generates self-contained HTML files

## Contributing

This tool is part of the Podman project's community engagement efforts. Contributions and improvements are welcome! Please follow the standard Podman contribution guidelines when submitting changes.

## License

This certificate generator is part of the Podman project and follows the same licensing terms. 
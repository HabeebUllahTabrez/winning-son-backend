# API Documentation Export Guide

This guide explains how to share your Winning Son API documentation with others.

## 📦 Available Export Formats

### 1. **Standalone HTML (Recommended for Non-Technical Users)**
**File:** `api-documentation-standalone.html` (14KB)

- ✅ Works offline - no internet required
- ✅ Interactive Swagger UI included
- ✅ Just open in any web browser
- ✅ Try API calls directly from the browser

**How to use:**
1. Send the file to anyone
2. They double-click to open in their browser
3. They can explore all endpoints interactively

---

### 2. **Postman Collection (Recommended for Developers)**
**File:** `Winning-Son-API.postman_collection.json` (11KB)

- ✅ Import into Postman
- ✅ All endpoints pre-configured
- ✅ Environment variables included
- ✅ Bearer token authentication ready

**How to import:**
1. Open Postman
2. Click "Import" button
3. Drag and drop the JSON file
4. Collection appears in left sidebar

**Usage:**
- Set the `jwt_token` variable after login
- Change `baseUrl` if API is hosted elsewhere
- All endpoints are organized by category

---

### 3. **OpenAPI/Swagger Files (For Technical Integration)**

#### **swagger.json** (30KB)
- Standard OpenAPI 2.0 format
- Machine-readable
- Can be imported into many tools

#### **swagger.yaml** (14KB)
- Same as JSON but more readable
- Better for version control
- Human-friendly format

**Compatible tools:**
- Swagger Editor (https://editor.swagger.io/)
- Insomnia REST Client
- Postman
- VS Code with Swagger Viewer extension
- API documentation generators
- Code generation tools (client SDKs)

---

## 🎯 Quick Start for Recipients

### Option A: Just Want to See the API?
**Send them:** `api-documentation-standalone.html`

They can:
1. Open it in Chrome/Firefox/Safari
2. Browse all endpoints
3. See request/response examples
4. Try API calls (if they have API access)

### Option B: Developer Wants to Test the API?
**Send them:**
1. `Winning-Son-API.postman_collection.json`
2. Tell them your API URL (e.g., `http://localhost:8080`)

They can:
1. Import into Postman
2. Update the base URL
3. Start testing immediately

### Option C: Need to Integrate or Generate Code?
**Send them:** `swagger.json` or `swagger.yaml`

They can:
1. Import into their API tool
2. Generate client libraries (many languages supported)
3. Build integrations
4. Create automated tests

---

## 📧 Email Templates

### For Non-Technical Stakeholders
```
Subject: Winning Son API Documentation

Hi [Name],

Attached is the interactive documentation for the Winning Son API.

To view:
1. Download the attached HTML file
2. Double-click to open in your browser
3. You can explore all API endpoints and see examples

The documentation includes:
- All available endpoints
- Request/response formats
- Authentication details
- Interactive examples

Let me know if you have any questions!
```

### For Developers
```
Subject: Winning Son API - Developer Documentation

Hi [Name],

Please find attached the API documentation in multiple formats:

1. api-documentation-standalone.html - Interactive web UI
2. Winning-Son-API.postman_collection.json - Import into Postman
3. swagger.json - OpenAPI specification for code generation

API Base URL: http://localhost:8080/api

Authentication: Bearer token (JWT)
- Get token from POST /api/auth/login

Quick start:
1. Import the Postman collection
2. Login to get your JWT token
3. Set the jwt_token variable in Postman
4. Start making requests

Full documentation: See the HTML file or swagger.json

Let me know if you need any help!
```

---

## 🌐 Hosting Options

### Option 1: GitHub Pages
1. Commit the `docs/` folder to your repo
2. Enable GitHub Pages in repo settings
3. Documentation is live at `https://your-username.github.io/your-repo/docs/api-documentation-standalone.html`

### Option 2: Cloud Storage
Upload `api-documentation-standalone.html` to:
- AWS S3 (make public)
- Google Cloud Storage
- Dropbox/Google Drive (share link)
- Netlify Drop (netlify.com/drop)

### Option 3: Swagger Hub
1. Go to https://app.swaggerhub.com/
2. Create free account
3. Upload `swagger.json`
4. Get shareable link

---

## 🔄 Keeping Documentation Updated

After making API changes:

```bash
# Regenerate Swagger docs
~/go/bin/swag init -g cmd/server/main.go -o ./docs

# Recreate standalone HTML
python3 << 'EOF'
import json
with open('docs/swagger.json', 'r') as f:
    spec = json.load(f)
with open('docs/api-documentation.html', 'r') as f:
    html = f.read()
html = html.replace('SWAGGER_SPEC_PLACEHOLDER', json.dumps(spec))
with open('docs/api-documentation-standalone.html', 'w') as f:
    f.write(html)
EOF

# Share updated files
```

---

## 📋 File Comparison

| File | Size | Best For | Requires Internet |
|------|------|----------|-------------------|
| `api-documentation-standalone.html` | 14KB | Quick sharing, demos | Yes (loads Swagger UI from CDN) |
| `Winning-Son-API.postman_collection.json` | 11KB | Testing, development | No |
| `swagger.json` | 30KB | Integration, code gen | No |
| `swagger.yaml` | 14KB | Version control, review | No |
| `SWAGGER.md` | 4.5KB | Usage guide | No |

---

## ❓ FAQ

**Q: Can I share just one file?**
A: Yes! The standalone HTML is the easiest single-file option.

**Q: Do they need Go installed?**
A: No, none of these require Go. They're just documentation.

**Q: Can they test the API from the HTML file?**
A: Yes, if they have network access to your API server.

**Q: How do I password-protect the documentation?**
A: Host it behind authentication (e.g., password-protected S3 bucket, private GitHub repo, or add basic auth to your deployment).

**Q: The HTML file loads slowly**
A: It loads Swagger UI from CDN. For offline use, you could download and bundle all assets.

**Q: Can I customize the appearance?**
A: Yes! Edit the `<style>` section in `api-documentation.html` and regenerate the standalone version.

---

## 🚀 Pro Tips

1. **Version your exports**: Include version number or date in filename
   - `api-documentation-v1.0-standalone.html`
   - `Winning-Son-API-2025-10-17.postman_collection.json`

2. **Create a docs package**: Zip all files together
   ```bash
   cd docs && zip ../api-docs-$(date +%Y%m%d).zip *.html *.json *.yaml SWAGGER.md
   ```

3. **Add examples**: Update Swagger annotations with example values for better documentation

4. **Include changelog**: Add a CHANGELOG.md in docs/ folder to track API changes

5. **Set up CI/CD**: Auto-generate and publish docs on every commit

---

## 📞 Support

For issues or questions about the API documentation:
- Check SWAGGER.md for detailed usage instructions
- Review the swagger.json for technical details
- Contact the API maintainer

---

**Last Updated:** October 2025
**API Version:** 1.0

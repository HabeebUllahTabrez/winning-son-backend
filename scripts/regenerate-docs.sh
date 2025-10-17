#!/bin/bash

# Script to regenerate all API documentation exports
# Usage: ./scripts/regenerate-docs.sh

set -e

echo "🔄 Regenerating API Documentation..."

# Step 1: Regenerate Swagger docs
echo "📝 Step 1/3: Generating Swagger specs..."
~/go/bin/swag init -g cmd/server/main.go -o ./docs

# Step 2: Create standalone HTML
echo "🌐 Step 2/3: Creating standalone HTML..."
python3 << 'EOF'
import json

# Read the swagger spec
with open('docs/swagger.json', 'r') as f:
    spec = json.load(f)

# Read the HTML template
with open('docs/api-documentation.html', 'r') as f:
    html = f.read()

# Replace the placeholder with the actual spec
html = html.replace('SWAGGER_SPEC_PLACEHOLDER', json.dumps(spec))

# Write the standalone file
with open('docs/api-documentation-standalone.html', 'w') as f:
    f.write(html)

print("✓ Standalone HTML created")
EOF

# Step 3: Create Postman collection
echo "📮 Step 3/3: Creating Postman collection..."
python3 << 'EOF'
import json
from datetime import datetime

# Read swagger spec
with open('docs/swagger.json', 'r') as f:
    swagger = json.load(f)

# Create Postman collection structure
collection = {
    "info": {
        "name": "Winning Son API",
        "description": swagger['info']['description'],
        "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
        "_exporter_id": "auto-generated"
    },
    "auth": {
        "type": "bearer",
        "bearer": [
            {
                "key": "token",
                "value": "{{jwt_token}}",
                "type": "string"
            }
        ]
    },
    "variable": [
        {
            "key": "baseUrl",
            "value": "http://localhost:8080/api",
            "type": "string"
        },
        {
            "key": "jwt_token",
            "value": "",
            "type": "string"
        }
    ],
    "item": []
}

# Group endpoints by tags
endpoints_by_tag = {}
for path, methods in swagger['paths'].items():
    for method, details in methods.items():
        tag = details.get('tags', ['Other'])[0]
        if tag not in endpoints_by_tag:
            endpoints_by_tag[tag] = []

        # Build request
        request = {
            "name": details.get('summary', path),
            "request": {
                "method": method.upper(),
                "header": [
                    {
                        "key": "Content-Type",
                        "value": "application/json"
                    }
                ],
                "url": {
                    "raw": "{{baseUrl}}" + path,
                    "host": ["{{baseUrl}}"],
                    "path": path.strip('/').split('/')
                },
                "description": details.get('description', '')
            },
            "response": []
        }

        # Add body if needed
        if method.upper() in ['POST', 'PUT', 'PATCH']:
            request["request"]["body"] = {
                "mode": "raw",
                "raw": json.dumps({}, indent=2)
            }

        # Add query params if any
        parameters = details.get('parameters', [])
        query_params = [p for p in parameters if p.get('in') == 'query']
        if query_params:
            request["request"]["url"]["query"] = [
                {
                    "key": p['name'],
                    "value": "",
                    "description": p.get('description', ''),
                    "disabled": not p.get('required', False)
                }
                for p in query_params
            ]

        endpoints_by_tag[tag].append(request)

# Create folders for each tag
for tag, requests in sorted(endpoints_by_tag.items()):
    collection["item"].append({
        "name": tag.capitalize(),
        "item": requests,
        "description": f"Endpoints related to {tag}"
    })

# Write the collection
with open('docs/Winning-Son-API.postman_collection.json', 'w') as f:
    json.dump(collection, f, indent=2)

print("✓ Postman collection created")
EOF

echo ""
echo "✅ All documentation regenerated successfully!"
echo ""
echo "📄 Generated files:"
echo "   • docs/swagger.json (OpenAPI spec)"
echo "   • docs/swagger.yaml (OpenAPI spec)"
echo "   • docs/api-documentation-standalone.html (Interactive UI)"
echo "   • docs/Winning-Son-API.postman_collection.json (Postman)"
echo ""
echo "🚀 View docs at: http://localhost:8080/swagger/index.html"
echo "📤 Share: Send any of the files above to your team"

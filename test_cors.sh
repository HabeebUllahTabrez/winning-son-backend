#!/bin/bash

echo "Testing CORS configuration..."

# Test preflight request
echo "1. Testing OPTIONS preflight request:"
curl -X OPTIONS \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type,Authorization" \
  -v http://localhost:8080/api/auth/login

echo -e "\n\n2. Testing actual request:"
curl -X GET \
  -H "Origin: http://localhost:3000" \
  -v http://localhost:8080/health

echo -e "\n\n3. Testing with different origin:"
curl -X GET \
  -H "Origin: http://localhost:5173" \
  -v http://localhost:8080/health

echo -e "\n\nCORS test completed!"

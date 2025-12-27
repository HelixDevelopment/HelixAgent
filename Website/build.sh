#!/bin/bash

# SuperAgent Website Build Script
# This script builds the website for production deployment

set -e

echo "🚀 Building SuperAgent Website..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Change to Website directory
cd Website

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    print_error "Node.js is not installed. Please install Node.js first."
    exit 1
fi

# Check if npm is installed
if ! command -v npm &> /dev/null; then
    print_error "npm is not installed. Please install npm first."
    exit 1
fi

print_status "Installing dependencies..."
npm install

print_status "Building website..."
npm run build

print_status "Optimizing images..."
# Create optimized images directory
mkdir -p public/assets/images/optimized

# Optimize images if imagemin-cli is available
if command -v imagemin &> /dev/null; then
    print_status "Optimizing images with imagemin..."
    find public/assets/images -name "*.png" -o -name "*.jpg" -o -name "*.jpeg" | while read -r image; do
        filename=$(basename "$image")
        imagemin "$image" --out-dir=public/assets/images/optimized --plugin=mozjpeg --plugin=pngquant
    done
    print_success "Images optimized successfully"
else
    print_warning "imagemin-cli not found. Skipping image optimization. Install with: npm install -g imagemin-cli imagemin-mozjpeg imagemin-pngquant"
fi

print_status "Minifying CSS..."
if command -v cssnano &> /dev/null; then
    cssnano public/styles/main.css public/styles/main.min.css
    print_success "CSS minified successfully"
else
    print_warning "cssnano not found. Skipping CSS minification. Install with: npm install -g cssnano postcss postcss-cli"
fi

print_status "Minifying JavaScript..."
if command -v terser &> /dev/null; then
    terser public/scripts/main.js -o public/scripts/main.min.js -c -m
    print_success "JavaScript minified successfully"
else
    print_warning "terser not found. Skipping JavaScript minification. Install with: npm install -g terser"
fi

print_status "Generating sitemap..."
cat > public/sitemap.xml << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
    <url>
        <loc>https://superagent.ai/</loc>
        <lastmod>$(date +%Y-%m-%d)</lastmod>
        <changefreq>weekly</changefreq>
        <priority>1.0</priority>
    </url>
    <url>
        <loc>https://superagent.ai/docs/</loc>
        <lastmod>$(date +%Y-%m-%d)</lastmod>
        <changefreq>weekly</changefreq>
        <priority>0.8</priority>
    </url>
    <url>
        <loc>https://superagent.ai/docs/api/</loc>
        <lastmod>$(date +%Y-%m-%d)</lastmod>
        <changefreq>weekly</changefreq>
        <priority>0.7</priority>
    </url>
</urlset>
EOF

print_status "Generating robots.txt..."
cat > public/robots.txt << 'EOF'
User-agent: *
Allow: /

Sitemap: https://superagent.ai/sitemap.xml
EOF

print_status "Adding security headers..."
cat > public/_headers << 'EOF'
/*
  X-Frame-Options: DENY
  X-Content-Type-Options: nosniff
  X-XSS-Protection: 1; mode=block
  Referrer-Policy: strict-origin-when-cross-origin
  Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline' https://cdnjs.cloudflare.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: https:;
EOF

print_status "Adding redirects for documentation..."
cat > public/_redirects << 'EOF'
# Redirect old URLs to new ones
/docs/api/ /docs/api/index.html 200
/docs/developer/ /docs/developer/index.html 200
/docs/user/ /docs/user/index.html 200
/docs/deployment/ /docs/deployment/index.html 200

# SPA redirects
/* /index.html 200
EOF

print_status "Creating build info..."
cat > public/build-info.json << EOF
{
  "buildDate": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "gitCommit": "$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')",
  "gitBranch": "$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo 'unknown')",
  "version": "1.0.0",
  "environment": "production"
}
EOF

print_status "Validating build..."
# Check if critical files exist
if [ ! -f "public/index.html" ]; then
    print_error "index.html not found in build output"
    exit 1
fi

if [ ! -f "public/styles/main.css" ]; then
    print_error "main.css not found in build output"
    exit 1
fi

if [ ! -f "public/scripts/main.js" ]; then
    print_error "main.js not found in build output"
    exit 1
fi

# Check file sizes (basic validation)
HTML_SIZE=$(wc -c < public/index.html)
CSS_SIZE=$(wc -c < public/styles/main.css)
JS_SIZE=$(wc -c < public/scripts/main.js)

if [ "$HTML_SIZE" -lt 1000 ]; then
    print_warning "index.html seems small ($HTML_SIZE bytes)"
fi

if [ "$CSS_SIZE" -lt 1000 ]; then
    print_warning "main.css seems small ($CSS_SIZE bytes)"
fi

if [ "$JS_SIZE" -lt 1000 ]; then
    print_warning "main.js seems small ($JS_SIZE bytes)"
fi

print_success "Build validation completed"

print_status "Build summary:"
echo "  📄 HTML: $(numfmt --to=iec $HTML_SIZE)"
echo "  🎨 CSS: $(numfmt --to=iec $CSS_SIZE)"
echo "  ⚡ JS: $(numfmt --to=iec $JS_SIZE)"

# Count total files
TOTAL_FILES=$(find public -type f | wc -l)
echo "  📁 Total files: $TOTAL_FILES"

print_success "Website build completed successfully! 🎉"
print_status "Build output available in: $(pwd)/public/"
print_status "Next steps:"
echo "  1. Test locally: cd Website && npm run serve"
echo "  2. Deploy to hosting platform"
echo "  3. Update DNS settings if needed"

# Return to original directory
cd ..

echo ""
print_success "Build process finished!"
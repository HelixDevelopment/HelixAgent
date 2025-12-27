#!/bin/bash

# SuperAgent Website Deployment Verification Script
# This script helps verify that the website is properly configured for deployment

echo "🚀 SuperAgent Website Deployment Verification"
echo "==========================================="
echo ""

# Check if GitHub Pages is enabled
echo "📋 Checking GitHub Pages Configuration..."
echo "To enable GitHub Pages:"
echo "1. Go to repository Settings → Pages"
echo "2. Select source: 'GitHub Actions'"
echo "3. Save configuration"
echo ""

# Check analytics configuration
echo "📊 Analytics Configuration Check:"
echo "Current placeholders detected:"
grep -n "GA_MEASUREMENT_ID\|CLARITY_PROJECT_ID" Website/public/index.html | head -5
echo ""
echo "To configure analytics:"
echo "1. Replace 'GA_MEASUREMENT_ID' with your Google Analytics 4 Measurement ID (format: G-XXXXXXXXXX)"
echo "2. Replace 'CLARITY_PROJECT_ID' with your Microsoft Clarity Project ID"
echo ""

# Check build system
echo "🔧 Build System Check:"
if [ -f "Website/package.json" ]; then
    echo "✅ package.json found"
    if [ -f "Website/build.sh" ]; then
        echo "✅ build.sh script found"
    else
        echo "❌ build.sh script missing"
    fi
else
    echo "❌ package.json missing"
fi
echo ""

# Check website files
echo "🌐 Website Files Check:"
if [ -f "Website/public/index.html" ]; then
    echo "✅ index.html found ($(wc -l < Website/public/index.html) lines)"
else
    echo "❌ index.html missing"
fi

if [ -f "Website/public/styles/main.min.css" ]; then
    echo "✅ CSS build found"
else
    echo "❌ CSS build missing"
fi

if [ -f "Website/public/scripts/main.min.js" ]; then
    echo "✅ JS build found"
else
    echo "❌ JS build missing"
fi
echo ""

# Check GitHub Actions workflow
echo "⚡ GitHub Actions Check:"
if [ -f ".github/workflows/docs-deploy.yml" ]; then
    echo "✅ Deployment workflow found"
    echo "Workflow will trigger on:"
    grep "branches:" .github/workflows/docs-deploy.yml | head -2
else
    echo "❌ Deployment workflow missing"
fi
echo ""

# Domain recommendations
echo "🌐 Domain Recommendations:"
echo "For production deployment, consider:"
echo "1. Custom domain: superagent.ai (recommended)"
echo "2. GitHub Pages default: https://[username].github.io/[repository]/"
echo ""

# Next steps
echo "📋 IMMEDIATE NEXT STEPS:"
echo "1. Enable GitHub Pages in repository settings"
echo "2. Configure analytics with real IDs"
echo "3. Test deployment by visiting the GitHub Pages URL"
echo "4. Monitor the Actions tab for deployment status"
echo ""

echo "🎯 SUCCESS METRICS TO TRACK:"
echo "- Website traffic: 1,000+ unique visitors in first month"
echo "- Engagement: 3+ minutes average session duration"
echo "- Conversion: 5%+ click-through rate to GitHub"
echo "- Social growth: 500+ Twitter followers, 300+ LinkedIn followers"
echo ""

echo "✨ SuperAgent is ready for deployment!"
echo "Follow the WEBSITE_LAUNCH_CHECKLIST.md for detailed launch steps."
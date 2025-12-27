# SuperAgent Analytics Configuration Script
# This script helps configure analytics with real tracking IDs

#!/bin/bash

echo "🚀 SuperAgent Analytics Configuration"
echo "===================================="
echo ""

# Check if we're in the right directory
if [ ! -f "Website/public/index.html" ]; then
    echo "❌ Error: Website/public/index.html not found"
    echo "Please run this script from the project root directory"
    exit 1
fi

# Function to replace analytics IDs
configure_analytics() {
    local ga_id="$1"
    local clarity_id="$2"
    
    echo "📊 Configuring Analytics..."
    echo "Google Analytics ID: $ga_id"
    echo "Microsoft Clarity ID: $clarity_id"
    echo ""
    
    # Backup original file
    cp Website/public/index.html Website/public/index.html.backup
    echo "✅ Created backup: Website/public/index.html.backup"
    
    # Replace GA Measurement ID
    sed -i "s/GA_MEASUREMENT_ID/$ga_id/g" Website/public/index.html
    echo "✅ Updated Google Analytics ID"
    
    # Replace Clarity Project ID
    sed -i "s/CLARITY_PROJECT_ID/$clarity_id/g" Website/public/index.html
    echo "✅ Updated Microsoft Clarity ID"
    
    # Verify changes
    echo ""
    echo "🔍 Verifying changes:"
    grep -n "G-" Website/public/index.html | head -2
    grep -n "clarity" Website/public/index.html | head -1
    
    echo ""
    echo "✅ Analytics configuration complete!"
}

# Function to test analytics
 test_analytics() {
    echo "🧪 Testing Analytics Configuration..."
    
    # Start local server
    echo "Starting local server..."
    cd Website
    python3 -m http.server 8080 --directory public &
    SERVER_PID=$!
    cd ..
    
    sleep 3
    
    # Test if server is running
    if curl -s http://localhost:8080 > /dev/null; then
        echo "✅ Local server is running"
        echo "🌐 Visit http://localhost:8080 to test analytics"
        echo ""
        echo "To test Google Analytics:"
        echo "1. Open browser console (F12)"
        echo "2. Look for 'Analytics Event' messages"
        echo "3. Check Network tab for analytics requests"
        echo ""
        echo "To test Microsoft Clarity:"
        echo "1. Visit your Clarity dashboard"
        echo "2. Look for real-time activity"
        echo "3. Check for heatmap data"
        echo ""
        echo "Press Ctrl+C to stop the server when done testing"
        
        # Keep server running
        wait $SERVER_PID
    else
        echo "❌ Failed to start local server"
        kill $SERVER_PID 2>/dev/null
    fi
}

# Function to revert changes
revert_changes() {
    echo "🔄 Reverting to backup..."
    if [ -f "Website/public/index.html.backup" ]; then
        cp Website/public/index.html.backup Website/public/index.html
        echo "✅ Reverted to backup configuration"
    else
        echo "❌ No backup file found"
    fi
}

# Main menu
echo "Select an option:"
echo "1. Configure Analytics (replace with real IDs)"
echo "2. Test Analytics Configuration"
echo "3. Revert to Backup"
echo "4. Setup Instructions"
echo ""

read -p "Enter your choice (1-4): " choice

case $choice in
    1)
        echo ""
        echo "📋 Google Analytics 4 Setup Instructions:"
        echo "1. Go to https://analytics.google.com/"
        echo "2. Create a new property for your website"
        echo "3. Copy the Measurement ID (format: G-XXXXXXXXXX)"
        echo ""
        read -p "Enter your GA4 Measurement ID: " ga_id
        
        echo ""
        echo "📋 Microsoft Clarity Setup Instructions:"
        echo "1. Go to https://clarity.microsoft.com/"
        echo "2. Create a new project for your website"
        echo "3. Copy the Project ID"
        echo ""
        read -p "Enter your Clarity Project ID: " clarity_id
        
        configure_analytics "$ga_id" "$clarity_id"
        
        echo ""
        echo "🎯 Next Steps:"
        echo "1. Commit your changes: git add Website/public/index.html"
        echo "2. Commit: git commit -m 'Configure analytics with real IDs'"
        echo "3. Push: git push origin main"
        echo "4. Test the live website after deployment"
        ;;
    2)
        test_analytics
        ;;
    3)
        revert_changes
        ;;
    4)
        echo ""
        echo "📚 Analytics Setup Instructions:"
        echo ""
        echo "Google Analytics 4:"
        echo "1. Visit https://analytics.google.com/"
        echo "2. Sign in with Google account"
        echo "3. Click 'Start measuring' or 'Admin' → 'Create Property'"
        echo "4. Enter property name: 'SuperAgent Website'"
        echo "5. Configure time zone and currency"
        echo "6. Click 'Create' → 'Web' → Enter website URL"
        echo "7. Copy Measurement ID (G-XXXXXXXXXX format)"
        echo ""
        echo "Microsoft Clarity:"
        echo "1. Visit https://clarity.microsoft.com/"
        echo "2. Sign in with Microsoft account"
        echo "3. Click 'New Project'"
        echo "4. Enter project name and website URL"
        echo "5. Copy the Project ID from installation instructions"
        echo ""
        echo "Privacy Considerations:"
        echo "- The website includes privacy-first analytics"
        echo "- Users can opt out of tracking"
        echo "- No personal data is collected"
        echo "- IP addresses are anonymized"
        echo "- Compliant with GDPR regulations"
        ;;
    *)
        echo "❌ Invalid choice. Please run the script again."
        ;;
esac

echo ""
echo "✨ Analytics configuration complete!"
echo "Check out ANALYTICS_CONFIGURATION_GUIDE.md for detailed instructions."
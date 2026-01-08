# HelixAgent Analytics Configuration Guide

## Overview
This guide will help you configure analytics for the HelixAgent website with privacy-first tracking and comprehensive monitoring.

## Step 1: Google Analytics 4 Setup

### Create GA4 Property
1. Go to [Google Analytics](https://analytics.google.com/)
2. Click "Admin" → "Create Property"
3. Enter property name: "HelixAgent Website"
4. Set time zone and currency
5. Click "Next" and complete setup

### Get Measurement ID
1. In your GA4 property, go to Admin → Data Streams
2. Click "Web" → "Create Stream"
3. Enter website URL: `https://helixagent.ai` (or your GitHub Pages URL)
4. Copy the Measurement ID (format: G-XXXXXXXXXX)

### Update Website
Replace `GA_MEASUREMENT_ID` in `Website/public/index.html`:

```html
<!-- Line 668 -->
<script async src="https://www.googletagmanager.com/gtag/js?id=G-XXXXXXXXXX"></script>

<!-- Line 673 -->
gtag('config', 'G-XXXXXXXXXX', {
```

## Step 2: Microsoft Clarity Setup

### Create Clarity Project
1. Go to [Microsoft Clarity](https://clarity.microsoft.com/)
2. Sign up/in with Microsoft account
3. Click "New Project"
4. Enter project name: "HelixAgent Website"
5. Enter website URL: `https://helixagent.ai`
6. Copy the Project ID

### Update Website
Replace `CLARITY_PROJECT_ID` in `Website/public/index.html`:

```javascript
// Line 687
})(window, document, "clarity", "script", "YOUR_PROJECT_ID");
```

## Step 3: Privacy-First Analytics

### GDPR Compliance
The website includes privacy-first analytics that:
- Don't track personal data
- Provide opt-out functionality
- Use session-based tracking
- Comply with privacy regulations

### Opt-Out Feature
Users can opt out of analytics:
```javascript
// Opt out
window.PrivacyAnalytics.optOut();

// Opt back in
window.PrivacyAnalytics.optIn();
```

## Step 4: Custom Events Tracking

### Pre-configured Events
The website tracks these events automatically:
- Page views
- CTA clicks
- Provider interactions
- Feature usage
- Performance metrics
- Error tracking

### Core Web Vitals
Monitors:
- Largest Contentful Paint (LCP)
- First Input Delay (FID)
- Cumulative Layout Shift (CLS)

### Business Events
Custom events for HelixAgent:
- Provider clicks (Claude, Gemini, etc.)
- Feature interactions
- Documentation views
- GitHub repository visits

## Step 5: Analytics Dashboard Setup

### Google Analytics Goals
Set up these conversion goals:
1. **GitHub Clicks**: Track clicks to GitHub repository
2. **Documentation Views**: Track visits to documentation
3. **Feature Engagement**: Track feature section interactions
4. **Provider Interest**: Track provider card clicks

### Clarity Heatmaps
Configure heatmaps for:
- Homepage engagement
- Feature section interaction
- CTA button effectiveness
- Navigation usage patterns

## Step 6: Performance Monitoring

### Success Metrics
Track these key metrics:
- **Traffic**: 1,000+ unique visitors/month
- **Engagement**: 3+ minutes average session
- **Conversion**: 5%+ GitHub click-through rate
- **Bounce Rate**: Keep below 40%
- **Mobile Traffic**: 40%+ of total

### Weekly Reports
Monitor:
- Traffic sources and channels
- Top performing content
- User behavior patterns
- Conversion funnel performance
- Geographic distribution

## Step 7: Privacy Considerations

### Data Minimization
- No personal data collection
- IP anonymization enabled
- Session-based tracking only
- User agent truncation

### Compliance Features
- GDPR-compliant tracking
- Opt-out functionality
- Clear privacy policy
- Data retention limits

## Step 8: Testing & Validation

### Test Analytics
1. Visit website after deployment
2. Check real-time analytics
3. Test custom events
4. Verify goal tracking
5. Check error tracking

### Validation Checklist
- [ ] GA4 property created and configured
- [ ] Clarity project set up
- [ ] Measurement IDs updated in website
- [ ] Privacy settings configured
- [ ] Goals and events tested
- [ ] Dashboard access verified

## Step 9: Ongoing Optimization

### Monthly Reviews
- Analyze traffic patterns
- Review conversion rates
- Optimize content based on data
- Adjust marketing strategies
- Update tracking as needed

### A/B Testing
Test different:
- Headlines and descriptions
- CTA button placements
- Feature presentations
- Navigation structures

## Quick Setup Commands

### Update Analytics IDs
```bash
# Backup original file
cp Website/public/index.html Website/public/index.html.backup

# Replace with your GA4 Measurement ID
sed -i 's/GA_MEASUREMENT_ID/G-XXXXXXXXXX/g' Website/public/index.html

# Replace with your Clarity Project ID  
sed -i 's/CLARITY_PROJECT_ID/YOUR_PROJECT_ID/g' Website/public/index.html

# Verify changes
grep -n "G-" Website/public/index.html | head -3
```

### Test Deployment
```bash
# Build and test locally
cd Website
npm run build
npm run serve

# Visit http://localhost:7061 and check browser console for analytics
```

## Support

For analytics setup help:
1. Check Google Analytics documentation
2. Review Microsoft Clarity guides
3. Test with browser developer tools
4. Monitor deployment logs

Remember: Analytics are essential for understanding user behavior and optimizing the website for better engagement and conversions.
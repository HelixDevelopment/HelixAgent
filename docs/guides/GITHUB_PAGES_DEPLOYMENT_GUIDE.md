# 🚀 HelixAgent GitHub Pages Deployment Guide

## QUICK SETUP INSTRUCTIONS

### Step 1: Enable GitHub Pages
1. Go to your GitHub repository: https://github.com/vasic-digital/HelixAgent
2. Click **Settings** (top right, gear icon)
3. Scroll down to **Pages** section in left sidebar
4. Under **Source**, select: **GitHub Actions**
5. Click **Save**

**Note**: It may take 1-2 minutes for the setting to take effect.

### Step 2: Configure Analytics
1. **Get Google Analytics 4 ID:**
   - Visit: https://analytics.google.com/
   - Create property: "HelixAgent Website"
   - Copy Measurement ID (format: G-XXXXXXXXXX)

2. **Get Microsoft Clarity ID:**
   - Visit: https://clarity.microsoft.com/
   - Create project with website URL
   - Copy Project ID from installation instructions

3. **Update Website with Real IDs:**
   ```bash
   # Run the configuration script
   ./configure-analytics.sh
   
   # Follow prompts to enter your IDs
   # The script will update Website/public/index.html
   
   # Commit and push changes
   git add Website/public/index.html
   git commit -m "Configure analytics with real IDs"
   git push origin main
   ```

### Step 3: Verify Deployment
1. After enabling GitHub Pages, wait 1-2 minutes
2. Visit your GitHub Pages URL (automatically generated):
   ```
   https://vasic-digital.github.io/HelixAgent/
   ```
3. Verify website loads correctly
4. Check browser console for analytics events

## TROUBLESHOOTING

### If GitHub Pages Doesn't Deploy:
1. **Check GitHub Actions tab:**
   - Go to https://github.com/vasic-digital/HelixAgent/actions
   - Look for "Deploy Documentation and Website" workflow
   - Check if it's running/completed/failed

2. **Common Issues:**
   - **Workflow not triggered**: Push a change to trigger it
   - **Build failure**: Check Actions logs for errors
   - **Permissions issue**: Ensure GitHub Pages is enabled

3. **Manual Trigger:**
   - Go to Actions → "Deploy Documentation and Website"
   - Click "Run workflow" button
   - Select "main" branch and run

### If Website Doesn't Load:
1. **Check URL format:**
   - Correct: `https://<username>.github.io/<repository>`
   - Example: `https://vasic-digital.github.io/HelixAgent/`

2. **Wait for propagation:**
   - GitHub Pages can take 1-10 minutes to deploy
   - Clear browser cache and try again

3. **Test locally:**
   ```bash
   cd Website
   npm run serve
   # Visit http://localhost:7061
   ```

## CUSTOM DOMAIN SETUP (Optional)

### Recommended Domain: helixagent.ai

1. **Purchase domain** (if not already owned)
   - Use domain registrar like Namecheap, Google Domains, etc.
   - Domain suggestion: helixagent.ai

2. **Configure DNS:**
   - Add CNAME record pointing to `vasic-digital.github.io`
   - Wait for DNS propagation (up to 48 hours)

3. **Update GitHub Pages:**
   - Go to Settings → Pages
   - Under "Custom domain", enter your domain
   - Check "Enforce HTTPS"

4. **Update Website Links:**
   - Update all references from GitHub Pages URL to custom domain
   - Update analytics configuration

## POST-DEPLOYMENT CHECKLIST

### ✅ Immediate Verification
- [ ] Website loads at GitHub Pages URL
- [ ] All links work correctly
- [ ] Mobile responsive design works
- [ ] Analytics tracking working (check browser console)
- [ ] Performance is good (use Lighthouse in Chrome DevTools)

### ✅ Analytics Configuration
- [ ] Google Analytics receiving data
- [ ] Microsoft Clarity heatmaps working
- [ ] Custom events tracking properly
- [ ] Privacy settings configured (GDPR compliant)

### ✅ SEO Verification
- [ ] Meta tags present and correct
- [ ] Structured data implemented
- [ ] Open Graph tags for social sharing
- [ ] Canonical URL set correctly
- [ ] Sitemap.xml accessible (if implemented)

## AUTOMATIC DEPLOYMENT

### How It Works
1. **Push to main branch** → Triggers workflow
2. **Website builds** → Creates optimized assets
3. **Documentation copies** → Adds docs to public folder
4. **Deploy to Pages** → Uploads to GitHub Pages
5. **Status updates** → Shows in Actions tab

### Manual Deployment
If automatic deployment doesn't work:

1. **Trigger manually:**
   ```
   git add .
   git commit -m "Trigger deployment"
   git push origin main
   ```

2. **Or use workflow dispatch:**
   - Go to Actions → "Deploy Documentation and Website"
   - Click "Run workflow"
   - Select "main" branch

## MONITORING

### GitHub Actions Status
- **URL**: https://github.com/vasic-digital/HelixAgent/actions
- **Workflow**: "Deploy Documentation and Website"
- **Frequency**: On push to main/master branches

### Performance Monitoring
- **Lighthouse Score**: Aim for 90+ on all metrics
- **Page Load Time**: Should be < 3 seconds
- **Mobile Responsiveness**: Test on various devices

### Analytics Dashboard
- **Google Analytics**: https://analytics.google.com/
- **Microsoft Clarity**: https://clarity.microsoft.com/
- **GitHub Insights**: Repository → Insights → Traffic

## TIPS FOR SUCCESS

### Before Going Live
1. **Test thoroughly** on different browsers and devices
2. **Check analytics** with real IDs before announcement
3. **Verify all links** work correctly
4. **Test mobile experience** on actual devices

### After Deployment
1. **Monitor traffic** for first 24 hours
2. **Check for errors** in browser console
3. **Test social sharing** (Open Graph previews)
4. **Verify search engine** indexing (use Google Search Console)

### Optimization
1. **Enable compression** (already done in build)
2. **Implement caching** (GitHub Pages handles this)
3. **Monitor performance** regularly
4. **Update content** as needed

## SUPPORT

### Common Issues & Solutions

**Issue**: "Page build failed"
**Solution**: Check GitHub Actions logs for specific error

**Issue**: Analytics not working
**Solution**: Verify Measurement IDs are correct, check browser console

**Issue**: Images not loading
**Solution**: Check file paths and permissions

**Issue**: Mobile layout broken
**Solution**: Test with different viewport sizes

### Getting Help
- **GitHub Issues**: Report deployment problems
- **GitHub Community**: Ask for help with GitHub Pages
- **Analytics Support**: Google/Microsoft documentation
- **Web Standards**: MDN Web Docs for technical issues

---

**🚀 READY TO LAUNCH?**

Once GitHub Pages is enabled and analytics configured, HelixAgent will be live at:
`https://vasic-digital.github.io/HelixAgent/`

**Next Steps:**
1. Enable GitHub Pages (Settings → Pages)
2. Configure analytics (run `./configure-analytics.sh`)
3. Test the live website
4. Begin marketing campaign!

**🎉 Congratulations! Your HelixAgent website is ready to go live!**
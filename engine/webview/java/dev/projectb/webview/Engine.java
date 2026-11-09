package dev.projectb.webview;

import android.annotation.SuppressLint;
import android.app.Activity;
import android.content.Context;
import android.net.http.SslError;
import android.os.Handler;
import android.os.Looper;
import android.view.ViewGroup;
import android.view.inputmethod.InputMethodManager;
import android.webkit.RenderProcessGoneDetail;
import android.webkit.SslErrorHandler;
import android.webkit.WebBackForwardList;
import android.webkit.WebChromeClient;
import android.webkit.WebHistoryItem;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.FrameLayout;
import org.json.JSONObject;

/**
 * The engine calls from Go, on Android System WebView. Two stacked views, as on the Mac: the page
 * at the bottom and the transparent shell on top. Each method is thin and only forwards.
 */
@SuppressLint("SetJavaScriptEnabled")
final class Engine implements ShellView.Host {
    // One tab so far.
    private static final long TAB = 1;

    private final Activity activity;
    private final FrameLayout root;
    private final float density;
    private final Handler main = new Handler(Looper.getMainLooper());

    private ShellView shell;
    private WebView page;
    private String startingUrl;
    private String lastUrl;
    // WebView commits an error page for a failed load and reports that commit like any other.
    private String failedUrl;

    Engine(Activity activity) {
        this.activity = activity;
        density = activity.getResources().getDisplayMetrics().density;
        root = new FrameLayout(activity);
        // A second finger on the bar during a pinch on the page must stay with the page.
        root.setMotionEventSplittingEnabled(false);
        shell = new ShellView(activity, this);
        root.addView(shell, matchParent());
    }

    ViewGroup root() {
        return root;
    }

    /** The phone's back key: the same command as the shell's back button. */
    boolean backKey() {
        if (!page.canGoBack()) {
            return false;
        }
        Core.shellMessage("{\"t\":\"back\"}");
        return true;
    }

    // Called from Go, through jni.c.

    long createTab() {
        page = newPage();
        root.addView(page, 0, matchParent());
        return TAB;
    }

    void loadUrl(long tab, String url) {
        if (allowed(url)) {
            page.loadUrl(url);
        }
    }

    void goBack(long tab) {
        move(-1);
    }

    void goForward(long tab) {
        move(1);
    }

    void reload(long tab) {
        // A page recreated after a crash has nothing to reload yet.
        String url = page.getUrl() != null ? page.getUrl() : lastUrl;
        if (url != null && allowed(url)) {
            if (page.getUrl() != null) {
                page.reload();
            } else {
                page.loadUrl(url);
            }
        }
    }

    void setPageBounds(double x, double y, double w, double h, double viewW, double viewH) {
        // Measured for a size the window no longer has. A fresher report follows.
        if (Math.abs(viewW - shell.getWidth() / density) > 1
                || Math.abs(viewH - shell.getHeight() / density) > 1) {
            return;
        }
        // Margins, not a size, so the page follows the window in the same frame when the
        // keyboard opens or the phone turns.
        FrameLayout.LayoutParams p = matchParent();
        p.leftMargin = px(x);
        p.topMargin = px(y);
        p.rightMargin = px(viewW - x - w);
        p.bottomMargin = px(viewH - y - h);
        page.setLayoutParams(p);
    }

    void setShellRegions(double[] rects) {
        shell.setRegions(rects);
    }

    void focusPage() {
        page.requestFocus();
        InputMethodManager ime =
                (InputMethodManager) activity.getSystemService(Context.INPUT_METHOD_SERVICE);
        ime.hideSoftInputFromWindow(root.getWindowToken(), 0);
    }

    void sendToShell(String json) {
        shell.evaluateJavascript("__shellReceive(" + JSONObject.quote(json) + ")", null);
    }

    // Any goroutine's thread.
    void postWakeup() {
        main.post(Core::runPosted);
    }

    // WebView only tells its client about navigations the page started, so the ones the app
    // starts ask the managers here. Content's navigation throttle sees both in one place.
    private boolean allowed(String url) {
        startingUrl = url;
        failedUrl = null;
        return Core.navigationStarting(TAB, url);
    }

    private void move(int delta) {
        WebBackForwardList list = page.copyBackForwardList();
        WebHistoryItem item = list.getItemAtIndex(list.getCurrentIndex() + delta);
        if (item != null && allowed(item.getUrl())) {
            page.goBackOrForward(delta);
        }
    }

    private int px(double css) {
        return (int) Math.round(css * density);
    }

    private static FrameLayout.LayoutParams matchParent() {
        return new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT);
    }

    private WebView newPage() {
        WebView view = new WebView(activity);
        WebSettings s = view.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        s.setAllowFileAccess(false);
        // Lay out pages without a mobile viewport the way a browser does, and let them be pinched.
        s.setUseWideViewPort(true);
        s.setLoadWithOverviewMode(true);
        s.setBuiltInZoomControls(true);
        s.setDisplayZoomControls(false);
        view.setWebViewClient(new PageClient());
        view.setWebChromeClient(new PageChromeClient());
        return view;
    }

    @Override
    public void shellRendererGone() {
        root.removeView(shell);
        shell.destroy();
        shell = new ShellView(activity, this);
        root.addView(shell, matchParent());
    }

    private void pageRendererGone() {
        ViewGroup.LayoutParams p = page.getLayoutParams();
        root.removeView(page);
        page.destroy();
        page = newPage();
        root.addView(page, 0, p);
        Core.pageCrashed(TAB);
        Core.historyChanged(TAB, false, false);
    }

    private final class PageClient extends WebViewClient {
        @Override
        public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
            return request.isForMainFrame() && !allowed(request.getUrl().toString());
        }

        // The one callback for every commit, same-document ones included.
        @Override
        public void doUpdateVisitedHistory(WebView view, String url, boolean isReload) {
            Core.historyChanged(TAB, view.canGoBack(), view.canGoForward());
            if (url.equals(failedUrl)) {
                failedUrl = null;
                return;
            }
            lastUrl = url;
            Core.navigationFinished(TAB, url);
            // A page coming back from history already has its title, and may not set it again.
            WebHistoryItem item = view.copyBackForwardList().getCurrentItem();
            if (item != null && item.getTitle() != null && !item.getTitle().isEmpty()) {
                Core.titleChanged(TAB, item.getTitle());
            }
        }

        @Override
        public void onReceivedError(WebView view, WebResourceRequest request, WebResourceError error) {
            if (request.isForMainFrame()) {
                failedUrl = request.getUrl().toString();
                Core.navigationFailed(TAB, failedUrl, error.getDescription().toString());
            }
        }

        // Refusing is WebView's default too, but left alone it says nothing and the old page stays.
        @Override
        public void onReceivedSslError(WebView view, SslErrorHandler handler, SslError error) {
            handler.cancel();
            if (error.getUrl().equals(startingUrl)) {
                Core.navigationFailed(TAB, startingUrl, "certificate error");
            }
        }

        @Override
        public boolean onRenderProcessGone(WebView view, RenderProcessGoneDetail detail) {
            pageRendererGone();
            return true;
        }
    }

    private final class PageChromeClient extends WebChromeClient {
        @Override
        public void onReceivedTitle(WebView view, String title) {
            Core.titleChanged(TAB, title);
        }

        // The only signal that covers the whole load: onPageStarted fires at commit, not at start.
        @Override
        public void onProgressChanged(WebView view, int progress) {
            Core.loadingChanged(TAB, progress < 100);
        }
    }
}

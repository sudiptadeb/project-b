package dev.projectb.webview;

import android.annotation.SuppressLint;
import android.content.Context;
import android.graphics.Color;
import android.net.Uri;
import android.os.Handler;
import android.os.Looper;
import android.view.MotionEvent;
import android.view.View;
import android.webkit.JavascriptInterface;
import android.webkit.RenderProcessGoneDetail;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import java.io.ByteArrayInputStream;
import java.io.IOException;

/** The Vue shell: a transparent WebView over the page that only takes touches on its own parts. */
@SuppressLint({"SetJavaScriptEnabled", "ViewConstructor"})
final class ShellView extends WebView {
    // Android reserves this host for apps to serve their own files on, so it never reaches the
    // network. https makes the shell a secure context that can load module scripts.
    private static final String HOST = "appassets.androidplatform.net";
    private static final String URL = "https://" + HOST + "/shell/index.html";

    private final float density;
    private double[] regions = new double[0];

    /** The bridge from the shell page, and what to do if the shell's renderer dies. */
    interface Host {
        void shellRendererGone();
    }

    ShellView(Context context, Host host) {
        super(context);
        density = context.getResources().getDisplayMetrics().density;
        setBackgroundColor(Color.TRANSPARENT);
        setOverScrollMode(View.OVER_SCROLL_NEVER);

        WebSettings s = getSettings();
        s.setJavaScriptEnabled(true);
        // The bar is sized in CSS pixels, so the phone's font size setting must not scale it.
        s.setTextZoom(100);

        addJavascriptInterface(new Bridge(), "ProjectBShell");
        setWebViewClient(new ShellClient(context, host));
        loadUrl(URL);
    }

    void setRegions(double[] rects) {
        regions = rects;
    }

    // Android offers a finger down to the views from the top, and the one that accepts it keeps
    // the whole gesture. Refusing here hands it to the page below, the same question the Mac
    // window's hitTest: asks.
    @Override
    public boolean dispatchTouchEvent(MotionEvent e) {
        if (e.getActionMasked() == MotionEvent.ACTION_DOWN && !inRegions(e.getX(), e.getY())) {
            return false;
        }
        return super.dispatchTouchEvent(e);
    }

    private boolean inRegions(float pxX, float pxY) {
        double x = pxX / density;
        double y = pxY / density;
        for (int i = 0; i + 3 < regions.length; i += 4) {
            if (x >= regions[i] && x < regions[i] + regions[i + 2]
                    && y >= regions[i + 1] && y < regions[i + 1] + regions[i + 3]) {
                return true;
            }
        }
        return false;
    }

    /** What the shell page calls. bridge.js is the only caller. */
    private static final class Bridge {
        private final Handler main = new Handler(Looper.getMainLooper());

        // WebView calls this on its own background thread, and the managers live on the main one.
        @JavascriptInterface
        public void send(String json) {
            main.post(() -> Core.shellMessage(json));
        }
    }

    /** Serves the shell from the APK's assets and never lets it navigate away. */
    private static final class ShellClient extends WebViewClient {
        private final Context context;
        private final Host host;

        ShellClient(Context context, Host host) {
            this.context = context;
            this.host = host;
        }

        @Override
        public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
            return true;
        }

        @Override
        public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest request) {
            Uri url = request.getUrl();
            if (HOST.equals(url.getHost())) {
                try {
                    String path = url.getPath().substring(1);
                    return new WebResourceResponse(mimeOf(path), "utf-8", context.getAssets().open(path));
                } catch (IOException e) {
                    // Falls through to the empty answer below.
                }
            }
            return new WebResourceResponse("text/plain", "utf-8", 404, "Not Found", null,
                    new ByteArrayInputStream(new byte[0]));
        }

        @Override
        public boolean onRenderProcessGone(WebView view, RenderProcessGoneDetail detail) {
            host.shellRendererGone();
            return true;
        }

        private static String mimeOf(String path) {
            if (path.endsWith(".html")) return "text/html";
            if (path.endsWith(".js")) return "text/javascript";
            if (path.endsWith(".css")) return "text/css";
            return "application/octet-stream";
        }
    }
}

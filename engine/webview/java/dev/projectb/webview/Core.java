package dev.projectb.webview;

/**
 * The Go core, reached through jni.c. Android has no main() of ours to link Go into, so this
 * load is what starts Go's runtime. Only the app's own process ever gets here: WebView's
 * renderer runs in a service belonging to the WebView package, which never loads our libraries.
 */
final class Core {
    static {
        System.loadLibrary("project_b_jni");
    }

    private Core() {}

    static native void start(Engine engine, String profileDir, String blockedHosts);

    static native void stop();

    static native void runPosted();

    static native void shellMessage(String json);

    static native boolean navigationStarting(long tab, String url);

    static native void navigationFinished(long tab, String url);

    static native void navigationFailed(long tab, String url, String reason);

    static native void historyChanged(long tab, boolean canGoBack, boolean canGoForward);

    static native void titleChanged(long tab, String title);

    static native void loadingChanged(long tab, boolean loading);

    static native void pageCrashed(long tab);
}

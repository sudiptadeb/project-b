package dev.projectb.browser;

import android.app.Activity;
import android.content.Context;
import android.graphics.PixelFormat;
import android.graphics.drawable.ColorDrawable;
import android.view.MotionEvent;
import android.view.SurfaceHolder;
import android.view.SurfaceView;
import android.view.ViewGroup;
import android.view.View;
import android.widget.FrameLayout;

import java.util.HashMap;
import java.util.Map;

import org.jni_zero.CalledByNative;
import org.jni_zero.JNINamespace;
import org.jni_zero.NativeMethods;

import org.chromium.components.embedder_support.view.ContentView;
import org.chromium.content_public.browser.Visibility;
import org.chromium.content_public.browser.WebContents;
import org.chromium.ui.base.ViewAndroidDelegate;
import org.chromium.ui.base.WindowAndroid;

/**
 * The window. One SurfaceView at the bottom, where the native compositor draws the shown page and the
 * shell, and above it a ContentView per tab and one for the shell, which draw nothing and only take
 * touches and the keyboard. Each ContentView sits where its page is drawn, so a touch reaches the
 * right WebContents with the right coordinates. Only the shown tab's view is visible.
 */
@JNINamespace("project_b")
final class BrowserWindow extends FrameLayout {
    private static final double AREA_PAGE = 0;

    private final Activity activity;
    private final WindowAndroid windowAndroid;
    private final float density;

    private long nativeWindow;
    private ContentView shellView;
    private WebContents shellContents;
    private final Map<WebContents, ContentView> pages = new HashMap<>();
    private WebContents shownContents;
    private ContentView shownView;
    private FrameLayout.LayoutParams pageParams = matchParent();
    // Five numbers a region, x, y, w, h and what a press there does, in order: the first that
    // holds a point decides. None holds it: the page.
    private double[] regions = new double[0];

    BrowserWindow(Activity activity, WindowAndroid windowAndroid) {
        super(activity);
        this.activity = activity;
        this.windowAndroid = windowAndroid;
        density = activity.getResources().getDisplayMetrics().density;
        // A second finger on the bar during a pinch on the page must stay with the page.
        setMotionEventSplittingEnabled(false);
    }

    /** Content is ready: make the surface, then the native side, which starts the app and Go. */
    void start(long goTable, String blockedHosts) {
        nativeWindow = BrowserWindowJni.get().start(this, windowAndroid, goTable, blockedHosts);
        SurfaceView surface = new SurfaceView(getContext());
        surface.setZOrderMediaOverlay(true);
        // A SurfaceView defaults to 16 bits per pixel.
        surface.getHolder().setFormat(PixelFormat.OPAQUE);
        surface.getHolder()
                .addCallback(
                        new SurfaceHolder.Callback() {
                            @Override
                            public void surfaceCreated(SurfaceHolder holder) {
                                BrowserWindowJni.get().surfaceCreated(nativeWindow);
                            }

                            @Override
                            public void surfaceChanged(
                                    SurfaceHolder holder, int format, int width, int height) {
                                BrowserWindowJni.get()
                                        .surfaceChanged(
                                                nativeWindow, format, width, height, holder.getSurface());
                            }

                            @Override
                            public void surfaceDestroyed(SurfaceHolder holder) {
                                BrowserWindowJni.get().surfaceDestroyed(nativeWindow);
                            }
                        });
        addView(surface, 0, matchParent());
        // Laid out before content was ready, so the size couldn't be told then.
        if (getWidth() > 0) {
            BrowserWindowJni.get().sizeChanged(nativeWindow, getWidth() / density, getHeight() / density);
        }
    }

    /** Hidden pages are told so, as they are when a tab is hidden. */
    void setActivityVisible(boolean visible) {
        int visibility = visible ? Visibility.VISIBLE : Visibility.HIDDEN;
        if (shellContents != null) {
            shellContents.updateWebContentsVisibility(visibility);
        }
        if (shownContents != null) {
            shownContents.updateWebContentsVisibility(visibility);
        }
    }

    @Override
    protected void onSizeChanged(int w, int h, int oldW, int oldH) {
        super.onSizeChanged(w, h, oldW, oldH);
        if (nativeWindow != 0) {
            BrowserWindowJni.get().sizeChanged(nativeWindow, w / density, h / density);
        }
    }

    @Override
    protected void onWindowVisibilityChanged(int visibility) {
        super.onWindowVisibilityChanged(visibility);
        windowAndroid.onVisibilityChanged(visibility == VISIBLE);
    }

    boolean backKey() {
        return nativeWindow != 0 && BrowserWindowJni.get().backKey(nativeWindow);
    }

    void stop() {
        if (nativeWindow != 0) {
            BrowserWindowJni.get().stop(nativeWindow);
        }
    }

    // The keyboard goes where the finger went. Each view takes focus for itself.
    @Override
    public boolean dispatchTouchEvent(MotionEvent e) {
        if (e.getActionMasked() == MotionEvent.ACTION_DOWN && shellView != null) {
            if (inShell(e.getX(), e.getY())) {
                shellView.requestFocus();
            } else if (shownView != null) {
                shownView.requestFocus();
            }
        }
        return super.dispatchTouchEvent(e);
    }

    /**
     * Holds the shell's view. The same question the Mac window's hitTest: asks: Android offers a
     * finger down to the views from the top, and refusing here hands it to the page below.
     */
    private final class ShellHolder extends FrameLayout {
        ShellHolder(Context context) {
            super(context);
        }

        @Override
        public boolean dispatchTouchEvent(MotionEvent e) {
            if (e.getActionMasked() == MotionEvent.ACTION_DOWN && !inShell(e.getX(), e.getY())) {
                return false;
            }
            return super.dispatchTouchEvent(e);
        }
    }

    // Nothing drags a phone's window, so whatever isn't the page is the shell.
    private boolean inShell(float pxX, float pxY) {
        double x = pxX / density;
        double y = pxY / density;
        for (int i = 0; i + 4 < regions.length; i += 5) {
            if (x >= regions[i] && x < regions[i] + regions[i + 2]
                    && y >= regions[i + 1] && y < regions[i + 1] + regions[i + 3]) {
                return regions[i + 4] != AREA_PAGE;
            }
        }
        return false;
    }

    @CalledByNative
    private void setShell(WebContents contents) {
        FrameLayout holder = new ShellHolder(getContext());
        shellContents = contents;
        shellView = attach(contents, holder, matchParent());
        addView(holder, matchParent());
        contents.updateWebContentsVisibility(Visibility.VISIBLE);
    }

    /** A new tab's view, hidden until showPage. The engine tells its page it's hidden. */
    @CalledByNative
    private void addPage(WebContents contents) {
        ContentView view = attach(contents, this, copy(pageParams));
        view.setVisibility(View.INVISIBLE);
        pages.put(contents, view);
    }

    @CalledByNative
    private void showPage(WebContents contents) {
        if (shownView != null) {
            shownView.setVisibility(View.INVISIBLE);
        }
        shownContents = contents;
        shownView = pages.get(contents);
        shownView.setVisibility(View.VISIBLE);
    }

    @CalledByNative
    private void removePage(WebContents contents) {
        ContentView view = pages.remove(contents);
        removeView(view);
        if (contents.equals(shownContents)) {
            shownContents = null;
            shownView = null;
        }
    }

    private ContentView attach(WebContents contents, ViewGroup parent, FrameLayout.LayoutParams params) {
        ContentView view = ContentView.createContentView(getContext(), contents);
        contents.setDelegates(
                "",
                ViewAndroidDelegate.createBasicDelegate(view),
                view,
                windowAndroid,
                WebContents.createDefaultInternalsHolder());
        view.addOnLayoutChangeListener(
                (v, left, top, right, bottom, oldLeft, oldTop, oldRight, oldBottom) -> {
                    contents.setSize(right - left, bottom - top);
                    BrowserWindowJni.get()
                            .contentsMoved(nativeWindow, contents, left, top, right - left, bottom - top);
                });
        // At the bottom: pages go under the shell, and the surface under them all.
        parent.addView(view, 0, params);
        return view;
    }

    @CalledByNative
    private void setLayout(
            double viewW, double viewH, double x, double y, double w, double h, double[] regions) {
        this.regions = regions;
        // Measured for a size the window no longer has. A fresher report follows.
        if (Math.abs(viewW - getWidth() / density) > 1 || Math.abs(viewH - getHeight() / density) > 1) {
            return;
        }
        // Margins, not a size, so the pages follow the window in the same frame when the keyboard
        // opens or the phone turns.
        pageParams = matchParent();
        pageParams.leftMargin = px(x);
        pageParams.topMargin = px(y);
        pageParams.rightMargin = px(viewW - x - w);
        pageParams.bottomMargin = px(viewH - y - h);
        for (ContentView view : pages.values()) {
            view.setLayoutParams(copy(pageParams));
        }
    }

    /** The frame colour, in the status bar and wherever a camera cutout keeps the window away. */
    @CalledByNative
    private void setFrameColor(int rgb) {
        int color = 0xff000000 | rgb;
        activity.getWindow().setBackgroundDrawable(new ColorDrawable(color));
        activity.getWindow().setStatusBarColor(color);
    }

    @CalledByNative
    private void minimize() {
        activity.moveTaskToBack(true);
    }

    @CalledByNative
    private void close() {
        activity.finish();
    }

    private int px(double css) {
        return (int) Math.round(css * density);
    }

    private static FrameLayout.LayoutParams copy(FrameLayout.LayoutParams p) {
        return new FrameLayout.LayoutParams(p);
    }

    private static FrameLayout.LayoutParams matchParent() {
        return new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT);
    }

    @NativeMethods
    interface Natives {
        long start(
                BrowserWindow self,
                WindowAndroid windowAndroid,
                long goTable,
                String blockedHosts);

        void surfaceCreated(long nativeBrowserWindow);

        void surfaceChanged(
                long nativeBrowserWindow,
                int format,
                int width,
                int height,
                android.view.Surface surface);

        void surfaceDestroyed(long nativeBrowserWindow);

        void sizeChanged(long nativeBrowserWindow, double width, double height);

        void contentsMoved(
                long nativeBrowserWindow,
                WebContents contents,
                int left,
                int top,
                int width,
                int height);

        boolean backKey(long nativeBrowserWindow);

        void stop(long nativeBrowserWindow);
    }
}

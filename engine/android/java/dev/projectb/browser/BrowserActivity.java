package dev.projectb.browser;

import android.app.Activity;
import android.os.Bundle;

import org.chromium.base.CommandLine;
import org.chromium.base.library_loader.LibraryLoader;
import org.chromium.base.library_loader.LibraryProcessType;
import org.chromium.content_public.browser.BrowserStartupController;
import org.chromium.ui.base.ActivityWindowAndroid;
import org.chromium.ui.base.IntentRequestTracker;

/** The main process's one window. Go's managers live exactly as long as it does. */
public final class BrowserActivity extends Activity {
    private BrowserWindow window;

    @Override
    protected void onCreate(Bundle saved) {
        super.onCreate(saved);
        if (!CommandLine.isInitialized()) {
            CommandLine.init(null);
        }
        LibraryLoader.getInstance().ensureInitialized();

        ActivityWindowAndroid windowAndroid =
                new ActivityWindowAndroid(
                        this,
                        /* listenToActivityState= */ true,
                        IntentRequestTracker.createFromActivity(this),
                        /* insetObserver= */ null,
                        /* occlusionTrackingAllowed= */ true);
        window = new BrowserWindow(this, windowAndroid);
        setContentView(window);

        // The same list the Mac app takes as --block.
        String blocked = getIntent().getStringExtra("block");
        long goTable = GoCore.table();
        BrowserStartupController.getInstance()
                .startBrowserProcessesAsync(
                        LibraryProcessType.PROCESS_BROWSER,
                        /* startGpuProcess= */ true,
                        /* startMinimalBrowser= */ false,
                        /* singleProcess= */ false,
                        new BrowserStartupController.StartupCallback() {
                            @Override
                            public void onSuccess(
                                    BrowserStartupController.StartupMetrics metrics) {
                                window.start(goTable, blocked == null ? "" : blocked);
                            }

                            @Override
                            public void onFailure() {
                                finish();
                            }
                        });
    }

    @Override
    protected void onStart() {
        super.onStart();
        window.setActivityVisible(true);
    }

    @Override
    protected void onStop() {
        super.onStop();
        window.setActivityVisible(false);
    }

    @Override
    public void onBackPressed() {
        if (!window.backKey()) {
            moveTaskToBack(true);
        }
    }

    // Go's state can't outlive the views it describes, so the process ends with the window.
    @Override
    protected void onDestroy() {
        window.stop();
        super.onDestroy();
        System.exit(0);
    }
}

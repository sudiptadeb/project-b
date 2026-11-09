package dev.projectb.webview;

import android.app.Activity;
import android.os.Bundle;
import java.io.File;

/** The window. Go's managers live exactly as long as it does. */
public final class BrowserActivity extends Activity {
    private Engine engine;

    @Override
    protected void onCreate(Bundle saved) {
        super.onCreate(saved);
        engine = new Engine(this);
        setContentView(engine.root());

        // The same list the Mac app takes as --block.
        String blocked = getIntent().getStringExtra("block");
        Core.start(engine, new File(getFilesDir(), "profile").getPath(), blocked == null ? "" : blocked);
    }

    @Override
    public void onBackPressed() {
        if (!engine.backKey()) {
            moveTaskToBack(true);
        }
    }

    // Go's state can't outlive the views it describes, so the process ends with the window.
    @Override
    protected void onDestroy() {
        Core.stop();
        super.onDestroy();
        System.exit(0);
    }
}

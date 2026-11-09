package dev.projectb.browser;

/**
 * The Go core. Loading this library is what starts Go's runtime, and that has to happen in the main
 * process only, so only BrowserActivity touches this class. An activity with no android:process
 * never runs in a renderer or a GPU process. Content's own loader doesn't know about this library,
 * so nothing loads it on a child's way up.
 */
final class GoCore {
    static {
        System.loadLibrary("project_b_go");
    }

    private GoCore() {}

    /** The address of the table of Go's functions, for the content library. */
    static native long table();
}

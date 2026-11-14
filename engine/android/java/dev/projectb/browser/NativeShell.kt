package dev.projectb.browser

import android.content.Context
import android.view.View

/**
 * A shell drawn by Android's own views instead of a page (docs/native-shell.md). It speaks the
 * shell's messages, the same JSON a shell page sends with chrome.send('shell') and hears through
 * __shellReceive: Go doesn't know which kind is on the other end.
 */
interface NativeShell {
    /** The shell's view, laid over the whole window, above the pages. */
    val view: View

    /** One message from Go. */
    fun receive(json: String)

    /** The keyboard for the shell: its address field. */
    fun focus()
}

/** How a native shell sends Go a message. */
fun interface ShellSender {
    fun send(json: String)
}

/** The native shells the app has, by the id the registry gives each (core/shells/registry.json). */
object NativeShells {
    @JvmStatic
    fun make(id: String, context: Context, send: ShellSender): NativeShell =
        when (id) {
            "mobile-native" -> MobileNativeShell(context, send)
            else -> throw IllegalArgumentException("no native shell $id")
        }
}

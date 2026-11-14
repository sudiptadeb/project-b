package dev.projectb.browser

import android.content.Context
import android.content.res.Configuration
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.net.Uri
import android.text.InputType
import android.util.TypedValue
import android.view.Gravity
import android.view.KeyEvent
import android.view.View
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.InputMethodManager
import android.widget.EditText
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.TextView
import org.json.JSONArray
import org.json.JSONObject

/**
 * The Mobile shell drawn with Android's views: "Mobile (native)", beside the web Mobile shell and
 * of the same design (docs/shells.md, "The mobile shell"), so the owner can compare the feel of
 * the two. Everything at the thumb: the page, and under it one 48dp bar with the address pill, the
 * tab count and the menu. The address bar can sit on top instead, as a setting has it.
 *
 * It is a shell without the SDK (docs/shells.md, "A shell without the SDK"): it keeps Go's state
 * from the snapshot and the messages after it, draws it, reports its layout in dp, which are the
 * web shell's CSS pixels on a phone, and gives a page the keyboard only once a layout shows it.
 */
class MobileNativeShell(context: Context, private val send: ShellSender) :
    FrameLayout(context), NativeShell {

    override val view: View
        get() = this

    private val density = resources.displayMetrics.density

    // Go's state, as the snapshot and the messages after it leave it.
    private var tabs = listOf<JSONObject>()
    private var active = 0L
    private var tones: JSONObject? = null
    private var theme = "system"
    private var addressBarPosition = "bottom"
    private var snapshotted = false
    private var shown = false

    // The tab last made active, so each change of it is seen once.
    private var lastActive = 0L
    private var editing = false
    private var lastLayout = ""
    // The tab the last layout sent shows: the only one the shell may give the keyboard.
    private var layoutShows = 0L

    private val toolbar = LinearLayout(context)
    private val hairline = View(context)
    private val pill = FrameLayout(context)
    private val address = AddressField(context)
    private val status = TextView(context)
    private val progress = View(context)
    private val count = CountButton(context)
    private val menu = DotsButton(context)

    init {
        // Draws nothing itself: the page shows wherever the bar isn't.
        setWillNotDraw(true)

        toolbar.orientation = LinearLayout.HORIZONTAL
        toolbar.gravity = Gravity.CENTER_VERTICAL
        toolbar.setPadding(dp(8), 0, dp(4), 0)

        address.apply {
            background = null
            setSingleLine(true)
            setTextSize(TypedValue.COMPLEX_UNIT_DIP, 14f)
            setPadding(dp(14), 0, dp(14), 0)
            gravity = Gravity.CENTER_VERTICAL
            hint = "Search or type an address"
            contentDescription = "Address"
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_URI or
                InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS
            imeOptions = EditorInfo.IME_ACTION_GO or EditorInfo.IME_FLAG_NO_EXTRACT_UI
            setSelectAllOnFocus(true)
            setOnFocusChangeListener { _, has -> if (has) begin() else end() }
            setOnEditorActionListener { _, action, event ->
                val enter = event?.keyCode == KeyEvent.KEYCODE_ENTER && event.action == KeyEvent.ACTION_DOWN
                if (action == EditorInfo.IME_ACTION_GO || enter) {
                    go(text.toString())
                    true
                } else {
                    false
                }
            }
        }
        status.apply {
            setTextSize(TypedValue.COMPLEX_UNIT_DIP, 12f)
            setPadding(0, 0, dp(14), 0)
            visibility = GONE
        }
        val pillRow = LinearLayout(context).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            addView(address, LinearLayout.LayoutParams(0, LayoutParams.MATCH_PARENT, 1f))
            addView(status, LinearLayout.LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.WRAP_CONTENT))
        }
        pill.addView(pillRow, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT))
        pill.addView(progress, LayoutParams(0, dp(2), Gravity.BOTTOM or Gravity.START))
        toolbar.addView(pill, LinearLayout.LayoutParams(0, dp(40), 1f).apply { marginEnd = dp(4) })

        // Every target 48dp, as on the web shell.
        count.contentDescription = "Tabs"
        menu.contentDescription = "Menu"
        toolbar.addView(count, LinearLayout.LayoutParams(dp(48), dp(48)))
        toolbar.addView(menu, LinearLayout.LayoutParams(dp(48), dp(48)))

        addView(toolbar, LayoutParams(LayoutParams.MATCH_PARENT, dp(48)))
        addView(hairline, LayoutParams(LayoutParams.MATCH_PARENT, 1))
        paint()
    }

    override fun onAttachedToWindow() {
        super.onAttachedToWindow()
        if (!snapshotted) send(JSONObject().put("t", "ready"))
    }

    // MARK: Go to the shell

    override fun receive(json: String) {
        val m = JSONObject(json)
        when (m.optString("t")) {
            "snapshot" -> {
                take(m)
                snapshotted = true
                draw()
            }
            "tabs", "theme", "windowColor", "addressBarPosition" -> {
                take(m)
                draw()
            }
            "tab" -> {
                val tab = m.getJSONObject("tab")
                val id = tab.getLong("id")
                tabs = tabs.map { if (it.getLong("id") == id) tab else it }
                draw()
            }
            "focusAddressBar" -> focus()
            // The phone's back key, while the shell said it closes something: the editing.
            "backKey" -> leave()
            "paint" -> send(JSONObject().put("t", "timeline").put("kind", "end").put("name", m.optString("span")))
        }
    }

    override fun focus() {
        address.requestFocus()
    }

    private fun take(m: JSONObject) {
        m.optJSONArray("tabs")?.let { list -> tabs = List(list.length()) { list.getJSONObject(it) } }
        if (m.has("active")) active = m.getLong("active")
        m.optJSONObject("tones")?.let { tones = it }
        if (m.has("theme")) theme = m.getString("theme")
        if (m.has("addressBarPosition")) addressBarPosition = m.getString("addressBarPosition")
    }

    private fun activeTab(): JSONObject? = tabs.firstOrNull { it.getLong("id") == active }

    // MARK: Drawing

    private fun draw() {
        val tab = activeTab()
        if (!editing) address.setText(host(tab?.optString("url") ?: ""))
        val state = when {
            tab == null -> ""
            tab.optBoolean("blocked") -> "Blocked"
            tab.optBoolean("crashed") -> "Crashed"
            tab.optString("error").isNotEmpty() -> "Failed"
            else -> ""
        }
        status.text = state
        status.visibility = if (state.isNotEmpty() && !editing) VISIBLE else GONE
        val loading = tab?.optBoolean("loading") == true && !editing
        progress.visibility = if (loading) VISIBLE else GONE
        if (loading) {
            val fraction = maxOf(tab!!.optDouble("progress", 0.0), 0.05)
            progress.layoutParams = (progress.layoutParams as LayoutParams).apply { width = (pill.width * fraction).toInt() }
        }
        count.count = tabs.size
        paint()
        placeBar()
        activated()
        report()
    }

    // The bar at the foot, or on top as the setting has it.
    private fun placeBar() {
        val top = addressBarPosition == "top"
        (toolbar.layoutParams as LayoutParams).gravity = if (top) Gravity.TOP else Gravity.BOTTOM
        (hairline.layoutParams as LayoutParams).apply {
            gravity = if (top) Gravity.TOP else Gravity.BOTTOM
            topMargin = if (top) dp(48) - 1 else 0
            bottomMargin = if (top) 0 else dp(48) - 1
        }
        toolbar.requestLayout()
        hairline.requestLayout()
    }

    // A tab that becomes active with nothing in it wants the address; one with a page wants
    // the page, once the layout shows it.
    private fun activated() {
        if (!snapshotted || active == lastActive) return
        lastActive = active
        if (activeTab()?.optString("url").isNullOrEmpty()) {
            focus()
        } else {
            address.clearFocus()
            post { focusPage() }
        }
    }

    // Go's tones, light or dark as the theme pref says, or the phone while it says system.
    private fun paint() {
        val dark = theme == "dark" || (theme == "system" &&
            (resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES)
        val palette = tones?.optJSONObject(if (dark) "dark" else "light") ?: return
        fun c(name: String) = parseColor(palette.optString(name))
        toolbar.setBackgroundColor(c("--bar"))
        hairline.setBackgroundColor(c("--line"))
        pill.background = GradientDrawable().apply {
            cornerRadius = dp(20).toFloat()
            setColor(c("--surface"))
        }
        progress.setBackgroundColor(c("--accent"))
        address.setTextColor(c("--text"))
        address.setHintTextColor(c("--muted"))
        status.setTextColor(c("--danger"))
        count.ink = c("--icon")
        menu.ink = c("--icon")
        count.press = c("--press")
        menu.press = c("--press")
    }

    override fun onConfigurationChanged(config: Configuration) {
        super.onConfigurationChanged(config)
        paint()
    }

    // MARK: Editing the address

    // A tap turns the pill into the field, with the whole address selected.
    private fun begin() {
        if (editing) return
        editing = true
        send(JSONObject().put("t", "focusShell"))
        send(JSONObject().put("t", "backCloses").put("on", true))
        address.setText(activeTab()?.optString("url") ?: "")
        address.selectAll()
        status.visibility = GONE
        progress.visibility = GONE
        imm().showSoftInput(address, InputMethodManager.SHOW_IMPLICIT)
    }

    private fun end() {
        if (!editing) return
        editing = false
        send(JSONObject().put("t", "backCloses").put("on", false))
        imm().hideSoftInputFromWindow(windowToken, 0)
        draw()
    }

    private fun go(text: String) {
        if (text.isBlank()) return
        send(JSONObject().put("t", "navigate").put("text", text.trim()))
        leave()
    }

    // Editing ends and the page has the keyboard.
    private fun leave() {
        address.clearFocus()
        end()
        focusPage()
    }

    private fun focusPage() {
        if (active != 0L && layoutShows == active) send(JSONObject().put("t", "focusTab").put("id", active))
    }

    private fun imm() = context.getSystemService(Context.INPUT_METHOD_SERVICE) as InputMethodManager

    // MARK: The layout, as the SDK sends it

    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        super.onLayout(changed, l, t, r, b)
        report()
    }

    private fun report() {
        if (!snapshotted || width == 0) return
        val w = width / density
        val h = height / density
        val bar = 48.0
        val top = addressBarPosition == "top"
        val barRect = rect(0.0, if (top) 0.0 else h - bar, w.toDouble(), bar)
        val tabsShown = JSONArray()
        if (active != 0L) {
            tabsShown.put(
                JSONObject().put("tab", active)
                    .put("rect", rect(0.0, if (top) bar else 0.0, w.toDouble(), h - bar)),
            )
        }
        val msg = JSONObject()
            .put("t", "layout")
            .put("shell", JSONArray().put(barRect))
            .put("controls", JSONArray())
            .put("drag", JSONArray())
            .put("tabs", tabsShown)
            .put("viewW", w.toDouble())
            .put("viewH", h.toDouble())
            .put("frame", 0)
        val json = msg.toString()
        if (json != lastLayout) {
            lastLayout = json
            layoutShows = active
            send.send(json)
        }
        if (!shown) {
            shown = true
            send(JSONObject().put("t", "shown"))
            if (activeTab()?.optString("url").isNullOrEmpty()) focus() else focusPage()
        }
    }

    private fun rect(x: Double, y: Double, w: Double, h: Double) =
        JSONObject().put("x", x).put("y", y).put("w", w).put("h", h)

    // MARK: Helpers

    private fun send(msg: JSONObject) = send.send(msg.toString())

    private fun dp(v: Int) = (v * density + 0.5f).toInt()

    /** What the pill shows of an address at rest: its host, as the web shell's does. */
    private fun host(url: String): String {
        if (url.isEmpty()) return ""
        val u = Uri.parse(url)
        return if ((u.scheme == "http" || u.scheme == "https") && u.host != null) {
            u.host!!.removePrefix("www.") + if (u.port != -1) ":${u.port}" else ""
        } else {
            url
        }
    }

    /** The address field: Enter goes, and its name is Address to a screen reader. */
    private class AddressField(context: Context) : EditText(context)

    /** The tab count: a rounded square with the number of tabs, :D past 99, as Chrome has it. */
    private inner class CountButton(context: Context) : PressButton(context) {
        var count = 0
            set(v) {
                field = v
                invalidate()
            }

        private val box = Paint(Paint.ANTI_ALIAS_FLAG).apply { style = Paint.Style.STROKE }
        private val digits = Paint(Paint.ANTI_ALIAS_FLAG).apply {
            textAlign = Paint.Align.CENTER
            typeface = Typeface.create(Typeface.DEFAULT, Typeface.BOLD)
        }

        override fun onDraw(canvas: Canvas) {
            super.onDraw(canvas)
            val side = 20 * density
            val cx = width / 2f
            val cy = height / 2f
            box.color = ink
            box.strokeWidth = 1.5f * density
            canvas.drawRoundRect(cx - side / 2, cy - side / 2, cx + side / 2, cy + side / 2, 5 * density, 5 * density, box)
            digits.color = ink
            digits.textSize = 11 * density
            canvas.drawText(if (count > 99) ":D" else count.toString(), cx, cy - (digits.descent() + digits.ascent()) / 2, digits)
        }
    }

    /** ⋯, three dots, as the web shell's icon. */
    private inner class DotsButton(context: Context) : PressButton(context) {
        private val dot = Paint(Paint.ANTI_ALIAS_FLAG)

        override fun onDraw(canvas: Canvas) {
            super.onDraw(canvas)
            dot.color = ink
            val r = 1.75f * density
            for (i in -1..1) canvas.drawCircle(width / 2f + i * 6 * density, height / 2f, r, dot)
        }
    }

    /** A 48dp round button: its ink, and the press colour behind it while held. */
    private open inner class PressButton(context: Context) : View(context) {
        var ink = Color.DKGRAY
            set(v) {
                field = v
                invalidate()
            }
        var press = Color.TRANSPARENT

        private val behind = Paint(Paint.ANTI_ALIAS_FLAG)

        init {
            isClickable = true
            isFocusable = false
            // A screen reader and the driver see a button.
            accessibilityDelegate = object : AccessibilityDelegate() {
                override fun onInitializeAccessibilityNodeInfo(host: View, info: android.view.accessibility.AccessibilityNodeInfo) {
                    super.onInitializeAccessibilityNodeInfo(host, info)
                    info.className = "android.widget.Button"
                }
            }
        }

        override fun onDraw(canvas: Canvas) {
            if (isPressed) {
                behind.color = press
                canvas.drawCircle(width / 2f, height / 2f, 20 * density, behind)
            }
        }

        override fun drawableStateChanged() {
            super.drawableStateChanged()
            invalidate()
        }
    }

    /** The root, to the driver: the native shell, as a page's WebView is to it a web one. */
    override fun getAccessibilityClassName(): CharSequence = "dev.projectb.browser.NativeShell"

    private fun parseColor(css: String): Int {
        if (css.startsWith("#")) return Color.parseColor(css)
        val m = Regex("""rgba\(\s*(\d+),\s*(\d+),\s*(\d+),\s*([\d.]+)\s*\)""").find(css) ?: return Color.TRANSPARENT
        val (r, g, b, a) = m.destructured
        return Color.argb((a.toFloat() * 255 + 0.5f).toInt(), r.toInt(), g.toInt(), b.toInt())
    }
}

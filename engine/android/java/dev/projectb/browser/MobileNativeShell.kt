package dev.projectb.browser

import android.annotation.SuppressLint
import android.content.Context
import android.content.res.Configuration
import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.Typeface
import android.text.Editable
import android.text.InputType
import android.text.TextWatcher
import android.util.TypedValue
import android.view.Gravity
import android.view.KeyEvent
import android.view.MotionEvent
import android.view.View
import android.view.ViewConfiguration
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.InputMethodManager
import android.widget.EditText
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.TextView
import org.json.JSONArray
import org.json.JSONObject
import kotlin.math.abs

/**
 * The Mobile shell drawn with Android's views: "Mobile (native)", beside the web Mobile shell and
 * of the same design (docs/shells.md, "The mobile shell", and the spec it was built from), so the
 * owner can compare the feel of the two. Everything at the thumb: the page, and under it one 48dp
 * bar with the address pill, the tab count and ⋯; the address bar can sit on top instead, as a
 * setting has it. Everything else is a sheet one of them opens: the switcher over the page, the
 * rest from the bottom.
 *
 * It is a shell without the SDK (docs/shells.md, "A shell without the SDK"): it keeps Go's state
 * from the snapshot and the messages after it, draws it, reports its layout in dp, which are the
 * web shell's CSS pixels on a phone, and gives a page the keyboard only once a layout shows it.
 */
@SuppressLint("ViewConstructor")
class MobileNativeShell(context: Context, private val sender: ShellSender) :
    FrameLayout(context), NativeShell {

    override val view: View
        get() = this

    private val density = resources.displayMetrics.density

    // Go's state, as the snapshot and the messages after it leave it.
    private var tabs = listOf<JSONObject>()
    private var active = 0L
    private var groups = listOf<JSONObject>()
    private var workspaces = listOf<JSONObject>()
    private var update: JSONObject? = null
    private var tones: JSONObject? = null
    private var theme = "system"
    private var addressBarPosition = "bottom"
    private var snapshotted = false
    private var shown = false
    private var palette = Palette(null)

    // The shell's own: what is open over the page, '' for nothing, 'switcher' or 'menu'; the
    // workspace new tabs open in and the switcher shows; whether the address is being edited.
    private var sheet = ""
    private var workspace = ""
    private var editing = false
    // The tab last made active, so each change of it is seen once.
    private var lastActive = 0L
    private var lastLayout = ""
    // The tab the last layout sent shows: the only one the shell may give the keyboard.
    private var layoutShows = 0L
    private var backCloses = false
    // The id of the last suggest sent: only its answer is listed.
    private var asked = 0
    // The last run's crash, "" when there was none or its notice was dismissed.
    private var crashLog = ""
    // What a long press in the page was on, for the "link" sheet; the group whose sheet is
    // open, and whether it opens asking to close it; the tab whose card menu is open.
    private var pressed: JSONObject? = null
    private var sheetGroup = ""
    private var askClose = false
    private var cardTab = 0L
    // The bars while the page scrolls down: away, sliding off the screen, then gone, their
    // room given to the page in one step, so the page is resized once and not every frame.
    // Back with the page scrolling up, another tab, the address or a sheet.
    private var away = false
    private var gone = false
    private val slide = if (reducedMotion(context)) 0L else 150L

    private val toolbar = Toolbar(context)
    private val pill = FrameLayout(context)
    // The lock, or a warning on plain http, before the host at rest.
    private val mark = IconView(context, "", 0, 16f)
    private val address = AddressField(context)
    private val status = TextView(context)
    private val clear = IconButton(context, "close", "Clear")
    private val progress = View(context)
    private val count = CountButton(context)
    private val menu = IconButton(context, "more", "Menu")
    private val suggestions = LinearLayout(context)
    private val switcher = Switcher(context, SwitcherActions())
    private val strip = Strip(context, StripActions())
    private val crashBar = CrashBar(context) { send(JSONObject().put("t", "dismissCrash")) }
    private val notice = PageNotice(context) {
        send(JSONObject().put("t", "reload"))
        focusPage()
    }
    // The bars: the address bar on top, when the setting has it there; and at the foot the
    // crash notice, the group strip, and the address bar.
    private val topStack = LinearLayout(context)
    private val bottomStack = LinearLayout(context)
    // Where a sheet goes, over everything.
    private val sheets = FrameLayout(context)

    init {
        setWillNotDraw(true)

        toolbar.setPadding(dp(8), 0, dp(4), 0)

        address.apply {
            background = null
            setSingleLine(true)
            setTextSize(TypedValue.COMPLEX_UNIT_DIP, 14f)
            setPadding(dp(14), 0, dp(8), 0)
            gravity = Gravity.CENTER_VERTICAL
            hint = "Search or type an address"
            contentDescription = "Address"
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_URI or InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS
            imeOptions = EditorInfo.IME_ACTION_GO or EditorInfo.IME_FLAG_NO_EXTRACT_UI
            setOnFocusChangeListener { _, has -> if (has) begin() else end() }
            setOnEditorActionListener { _, action, event ->
                val enter = event?.keyCode == KeyEvent.KEYCODE_ENTER && event.action == KeyEvent.ACTION_DOWN
                if (action == EditorInfo.IME_ACTION_GO || enter) { go(text.toString()); true } else false
            }
            addTextChangedListener(object : TextWatcher {
                override fun beforeTextChanged(s: CharSequence?, a: Int, b: Int, c: Int) {}
                override fun onTextChanged(s: CharSequence?, a: Int, b: Int, c: Int) {}
                override fun afterTextChanged(s: Editable?) { if (editing) typed() }
            })
        }
        status.apply {
            setTextSize(TypedValue.COMPLEX_UNIT_DIP, 12f)
            setPadding(0, 0, dp(14), 0)
            visibility = GONE
        }
        clear.iconSize = 18f
        clear.visibility = GONE
        clear.setOnClickListener { address.setText(""); address.requestFocus() }
        val pillRow = LinearLayout(context).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            addView(mark, LinearLayout.LayoutParams(dp(16), dp(16)).apply { marginStart = dp(12); marginEnd = -dp(6) })
            addView(address, LinearLayout.LayoutParams(0, LayoutParams.MATCH_PARENT, 1f))
            addView(status, LinearLayout.LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.WRAP_CONTENT))
            addView(clear, LinearLayout.LayoutParams(dp(40), dp(40)))
        }
        pill.addView(pillRow, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT))
        pill.addView(progress, LayoutParams(0, dp(2), Gravity.BOTTOM or Gravity.START))
        pill.clipToOutline = true
        toolbar.addView(pill, LinearLayout.LayoutParams(0, dp(40), 1f).apply { marginEnd = dp(4) })
        count.setOnClickListener { open(if (sheet == "switcher") "" else "switcher") }
        menu.setOnClickListener { open(if (sheet == "menu") "" else "menu") }
        toolbar.addView(count, LinearLayout.LayoutParams(dp(48), dp(48)))
        toolbar.addView(menu, LinearLayout.LayoutParams(dp(48), dp(48)))

        suggestions.orientation = LinearLayout.VERTICAL
        suggestions.visibility = GONE
        suggestions.isClickable = true
        switcher.visibility = GONE

        notice.visibility = GONE
        strip.visibility = GONE
        crashBar.visibility = GONE
        topStack.orientation = LinearLayout.VERTICAL
        bottomStack.orientation = LinearLayout.VERTICAL
        bottomStack.addView(crashBar, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, dp(48) + 1))
        bottomStack.addView(strip, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, dp(48) + 1))
        bottomStack.addView(toolbar, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, dp(48) + 1))

        // In drawing order: the switcher over the group strip, which it hides as the web
        // shell's does, but not over the address bar, which it leaves room for.
        addView(notice, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT))
        addView(topStack, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT, Gravity.TOP))
        addView(bottomStack, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT, Gravity.BOTTOM))
        addView(switcher, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT))
        addView(suggestions, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT))
        addView(sheets, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT))
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
            "tabs", "theme", "windowColor", "addressBarPosition", "workspaces", "update", "crashLog" -> {
                take(m)
                draw()
            }
            "tab" -> {
                val tab = m.getJSONObject("tab")
                val id = tab.getLong("id")
                tabs = tabs.map { if (it.getLong("id") == id) tab else it }
                draw()
            }
            "focusAddressBar" -> {
                open("")
                bars(true)
                focus()
            }
            "scroll" -> {
                if (m.optLong("tab") == active && sheet.isEmpty() && !editing) bars(m.optString("direction") != "down")
            }
            "contextMenu" -> if (m.optLong("tab") == active) {
                pressed = m
                open("link")
            }
            "suggestions" -> if (m.optInt("id") == asked && editing) suggest(m.optJSONArray("items") ?: JSONArray())
            // The phone's back key, while the shell said it closes something: the sheet open,
            // or the editing.
            "backKey" -> if (sheet.isNotEmpty()) open("") else leave()
            "paint" -> send(JSONObject().put("t", "timeline").put("kind", "end").put("name", m.optString("span")))
        }
    }

    override fun focus() {
        address.requestFocus()
    }

    private fun take(m: JSONObject) {
        fun list(name: String) = m.optJSONArray(name)?.let { a -> List(a.length()) { a.getJSONObject(it) } }
        list("tabs")?.let { tabs = it }
        if (m.has("active")) active = m.getLong("active")
        list("groups")?.let { groups = it }
        list("workspaces")?.let { workspaces = it }
        if (m.has("update")) update = m.optJSONObject("update")
        m.optJSONObject("tones")?.let { tones = it }
        if (m.has("theme")) theme = m.getString("theme")
        if (m.has("addressBarPosition")) addressBarPosition = m.getString("addressBarPosition")
        if (m.has("crashLog")) crashLog = m.optString("crashLog")
    }

    private fun activeTab(): JSONObject? = tabs.firstOrNull { it.getLong("id") == active }

    // MARK: Drawing

    private fun draw() {
        palette = Palette(tones?.optJSONObject(if (dark()) "dark" else "light"))
        val tab = activeTab()
        val url = tab?.optString("url") ?: ""
        if (!editing) address.setText(host(url))
        val state = when {
            tab == null -> ""
            tab.optBoolean("blocked") -> "Blocked"
            tab.optBoolean("crashed") -> "Crashed"
            tab.optString("error").isNotEmpty() -> "Failed"
            else -> ""
        }
        status.text = state
        status.visibility = if (state.isNotEmpty() && !editing) VISIBLE else GONE
        val secure = when {
            url.startsWith("https:") -> "lock"
            url.startsWith("http:") -> "warn"
            else -> ""
        }
        mark.visibility = if (secure.isNotEmpty() && !editing) VISIBLE else GONE
        mark.icon = secure
        mark.color = palette.icon
        clear.visibility = if (editing && address.text.isNotEmpty()) VISIBLE else GONE
        val loading = tab?.optBoolean("loading") == true && !editing
        progress.visibility = if (loading) VISIBLE else GONE
        if (loading) {
            val fraction = maxOf(tab!!.optDouble("progress", 0.0), 0.05)
            progress.layoutParams = (progress.layoutParams as LayoutParams).apply { width = (pill.width * fraction).toInt() }
        }
        count.count = tabs.size
        count.isSelected = sheet == "switcher"
        paint()
        val group = activeGroup()
        strip.visibility = if (group != null) VISIBLE else GONE
        if (group != null) strip.show(palette, group, Switcher.memberTabs(group, tabs), active)
        crashBar.visibility = if (crashLog.isNotEmpty()) VISIBLE else GONE
        if (crashLog.isNotEmpty()) crashBar.show(palette, crashLog)
        val failed = tab != null && url.isNotEmpty() && !tab.optBoolean("loading") &&
            (tab.optBoolean("crashed") || tab.optString("error").isNotEmpty() || tab.optBoolean("blocked"))
        notice.visibility = if (failed) VISIBLE else GONE
        if (failed) notice.show(palette, tab!!)
        placeBar()
        if (sheet == "switcher") {
            switcher.show(palette, tabs, active, groups, workspaces, workspace)
        }
        // A page that starts loading in the active tab brings the bars back, as Chrome's do.
        val now = if (tab?.optBoolean("loading") == true) url else ""
        if (now.isNotEmpty() && now != loadingUrl) bars(true)
        loadingUrl = now
        activated()
        report()
    }

    // The address the active tab was last seen loading, so each new load is seen once.
    private var loadingUrl = ""

    private fun activeGroup() = groups.firstOrNull { Switcher.inGroup(it, active) }

    // The bars slide away while the page scrolls down, then give their room to the page.
    private fun bars(show: Boolean) {
        if (show != away) return
        val stacks = listOf(topStack, bottomStack)
        if (show) {
            away = false
            gone = false
            for (s in stacks) s.visibility = VISIBLE
            report()
            for (s in stacks) s.animate().translationY(0f).setDuration(slide).start()
        } else {
            away = true
            topStack.animate().translationY(-topStack.height.toFloat()).setDuration(slide).start()
            bottomStack.animate().translationY(bottomStack.height.toFloat()).setDuration(slide).withEndAction {
                if (!away) return@withEndAction
                gone = true
                for (s in stacks) s.visibility = INVISIBLE
                report()
            }.start()
        }
    }

    private fun dark() = theme == "dark" || (theme == "system" &&
        (resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES)

    // The bar at the foot, or on top as the setting has it; the switcher and the suggestions
    // between it and the page's other edge.
    private fun placeBar() {
        val top = addressBarPosition == "top"
        val home = if (top) topStack else bottomStack
        if (toolbar.parent !== home) {
            (toolbar.parent as? LinearLayout)?.removeView(toolbar)
            home.addView(toolbar, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, dp(48) + 1))
        }
        toolbar.lineOnTop = !top
        val h = heights()
        // The switcher between the bars, over the group strip; the suggestions against the
        // address bar; the notice over the page.
        (switcher.layoutParams as LayoutParams).apply {
            topMargin = h.top
            bottomMargin = h.bottom - h.strip
        }
        (suggestions.layoutParams as LayoutParams).apply {
            gravity = if (top) Gravity.TOP else Gravity.BOTTOM
            topMargin = if (top) h.top else 0
            bottomMargin = if (top) 0 else h.bottom
        }
        (notice.layoutParams as LayoutParams).apply {
            topMargin = h.top
            bottomMargin = h.bottom
        }
        requestLayout()
    }

    // The bars' heights in px, as they show: none while they are gone.
    private class Heights(val top: Int, val bottom: Int, val strip: Int)

    private fun heights(): Heights {
        if (gone) return Heights(0, 0, 0)
        val bar = dp(48) + 1
        val top = addressBarPosition == "top"
        val strip = if (activeGroup() != null) bar else 0
        val crash = if (crashLog.isNotEmpty()) bar else 0
        return Heights(if (top) bar else 0, (if (top) 0 else bar) + strip + crash, strip)
    }

    // A tab that becomes active with nothing in it wants the address; one with a page wants
    // the page, once the layout shows it.
    private fun activated() {
        if (!snapshotted || active == lastActive) return
        lastActive = active
        bars(true)
        workspace = activeTab()?.optString("workspace") ?: ""
        if (activeTab()?.optString("url").isNullOrEmpty()) {
            focus()
        } else {
            address.clearFocus()
            post { focusPage() }
        }
    }

    // Go's tones, light or dark as the theme pref says, or the phone while it says system.
    private fun paint() {
        toolbar.setBackgroundColor(palette.bar)
        toolbar.line = palette.line
        pill.background = rounded(palette.surface, dpf(20))
        progress.setBackgroundColor(palette.accent)
        address.setTextColor(palette.text)
        address.setHintTextColor(palette.muted)
        status.setTextColor(palette.danger)
        for (b in listOf(count, menu, clear)) {
            b.ink = palette.icon
            b.press = palette.press
        }
        suggestions.setBackgroundColor(palette.surface)
    }

    override fun onConfigurationChanged(config: Configuration) {
        super.onConfigurationChanged(config)
        draw()
    }

    // MARK: Sheets

    // Opens what is over the page, or closes it with "". The back key closes it first.
    private fun open(name: String) {
        sheet = name
        if (name.isNotEmpty() && editing) leave()
        sheets.removeAllViews()
        switcher.visibility = if (name == "switcher") VISIBLE else GONE
        if (name == "switcher") workspace = activeTab()?.optString("workspace") ?: ""
        if (name.isNotEmpty()) bars(true)
        val made = when (name) {
            "menu" -> menuSheet()
            "group" -> groupSheetView()
            "card" -> cardSheet()
            "link" -> linkSheetView()
            else -> null
        }
        if (made != null) sheets.addView(made, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT))
        backClosesNow()
        draw()
    }

    private fun backClosesNow() {
        val on = sheet.isNotEmpty() || editing
        if (on != backCloses) {
            backCloses = on
            send(JSONObject().put("t", "backCloses").put("on", on))
        }
    }

    // The ⋯ sheet: a row of the page's actions, Back while there is somewhere to go back to
    // (the phone's back key does it too), Forward, Reload or Stop, New tab, and New tab in
    // group while the tab is in one; then Workspaces, where new tabs open, while there are two
    // or more, the update while one is ready, and Settings. Each acts and closes the sheet.
    private fun menuSheet(): View {
        val s = Sheet(context, "Menu", palette) { open("") }
        val tab = activeTab()
        val group = groups.firstOrNull { Switcher.inGroup(it, active) }
        val actions = LinearLayout(context).apply {
            orientation = LinearLayout.HORIZONTAL
            setPadding(dp(8), dp(4), dp(8), dp(8))
        }
        fun action(icon: String, name: String, words: String, enabled: Boolean = true, run: () -> Unit) {
            val b = LinearLayout(context).apply {
                orientation = LinearLayout.VERTICAL
                gravity = Gravity.CENTER_HORIZONTAL
                isClickable = enabled
                isEnabled = enabled
                setOnClickListener { run(); if (sheet == "menu") open("") }
                background = android.graphics.drawable.StateListDrawable().apply {
                    addState(intArrayOf(android.R.attr.state_pressed), rounded(palette.press, dpf(12)))
                }
            }
            asButton(b, name)
            val ink = if (enabled) palette.icon else Palette.alpha(palette.icon, 0.38f)
            b.addView(IconView(context, icon, ink), LinearLayout.LayoutParams(dp(48), dp(40)))
            b.addView(label(context, 12f, if (enabled) palette.muted else Palette.alpha(palette.muted, 0.5f)).apply { text = words; gravity = Gravity.CENTER })
            actions.addView(b, LinearLayout.LayoutParams(0, dp(64), 1f))
        }
        if (tab?.optBoolean("canGoBack") == true) action("back", "Back", "Back") { send(JSONObject().put("t", "back")) }
        action("forward", "Forward", "Forward", tab?.optBoolean("canGoForward") == true) { send(JSONObject().put("t", "forward")) }
        if (tab?.optBoolean("loading") == true) {
            action("stop", "Stop", "Stop") { send(JSONObject().put("t", "run").put("text", "nav.stop")) }
        } else {
            action("reload", "Reload", "Reload", !tab?.optString("url").isNullOrEmpty()) { send(JSONObject().put("t", "reload")) }
        }
        action("plus", "New Tab", "New tab") { newTab(0) }
        if (group != null) action("group", "New tab in group", "In group") { newTab(active) }
        s.content.addView(actions)
        s.content.addView(View(context).apply { setBackgroundColor(palette.line) }, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, 1))
        if (workspaces.size > 1) {
            s.content.addView(sheetRow(context, palette, "workspace", "Workspaces", "chevron") { workspaceList(s) })
        }
        if (update?.optString("phase") == "ready") {
            s.content.addView(sheetRow(context, palette, "update", "Install update to ${update?.optString("version")}") {
                send(JSONObject().put("t", "relaunch")); open("")
            })
        }
        s.content.addView(sheetRow(context, palette, "settings", "Settings") {
            open("")
            send(JSONObject().put("t", "run").put("text", "app.settings"))
        })
        return s
    }

    // Workspaces, in the menu sheet: where new tabs open, a tick on the one chosen.
    private fun workspaceList(s: Sheet) {
        s.content.removeAllViews()
        for (w in workspaces) {
            val id = w.optString("id")
            s.content.addView(sheetRow(context, palette, null, w.optString("name")) {
                workspace = id
                open("")
            }.also { row ->
                val dot = View(context).apply { background = android.graphics.drawable.GradientDrawable().apply { shape = android.graphics.drawable.GradientDrawable.OVAL; setColor(Palette.parseCss(w.optString("color", "#9aa0a6"))) } }
                (row as LinearLayout).addView(dot, 0, LinearLayout.LayoutParams(dp(10), dp(10)).apply { marginEnd = dp(16) })
                if (id == workspace) row.addView(label(context, 14f, palette.accent).apply { text = "✓" })
                row.isSelected = id == workspace
            })
        }
    }

    private inner class SwitcherActions : Switcher.Actions {
        override fun pick(id: Long) = this@MobileNativeShell.pick(id)
        override fun closeTab(id: Long) = closeTabs(listOf(id))
        override fun newTab() = this@MobileNativeShell.newTab(0)
        override fun card(id: Long) {
            cardTab = id
            open("card")
        }
        override fun group(id: String) = openGroup(id, false)
        override fun closeGroup(id: String) = openGroup(id, true)
        override fun workspace(id: String) {
            workspace = id
            draw()
        }
    }

    private inner class StripActions : Strip.Actions {
        override fun groupSheet() = openGroup(activeGroup()?.getString("id") ?: "", false)
        override fun pick(id: Long) = this@MobileNativeShell.pick(id)
        override fun closeTab(id: Long) = closeTabs(listOf(id))
        override fun newTabInGroup() = newTab(active)
    }

    private fun openGroup(id: String, ask: Boolean) {
        sheetGroup = id
        askClose = ask
        open("group")
    }

    // A group's sheet: its name, colour, tabs, New tab in it, Ungroup, and Close group.
    private fun groupSheetView(): View? {
        val g = groups.firstOrNull { it.getString("id") == sheetGroup } ?: return null
        val members = Switcher.memberTabs(g, tabs)
        val s = Sheet(context, "Group", palette) { open("") }
        groupSheet(s, palette, g, members, askClose,
            change = { fields -> fields.put("t", "editGroup").put("group", sheetGroup); send(fields) },
            pick = { pick(it) },
            closeTab = { closeTabs(listOf(it)) },
            newTab = { newTab(members.lastOrNull()?.getLong("id") ?: active) },
            ungroup = { send(JSONObject().put("t", "ungroup").put("group", sheetGroup)); open("") },
            closeGroup = {
                open("")
                if (members.size >= tabs.size) send(JSONObject().put("t", "newTab").put("workspace", workspace))
                send(JSONObject().put("t", "closeGroup").put("group", g.getString("id")))
            })
        return s
    }

    // A card's menu: Add to group, Remove from group while it is in one, and Close.
    private fun cardSheet(): View? {
        val tab = tabs.firstOrNull { it.getLong("id") == cardTab } ?: return null
        val own = groups.firstOrNull { Switcher.inGroup(it, cardTab) }
        val same = groups.filter { it !== own && (Switcher.memberTabs(it, tabs).firstOrNull()?.optString("workspace") ?: "") == tab.optString("workspace") }
        val s = Sheet(context, "Tab", palette) { open("") }
        cardMenu(s, palette, tab, own != null, same,
            add = { send(JSONObject().put("t", "addToGroup").put("id", cardTab).put("group", it)); open("switcher") },
            newGroup = { newTab(cardTab) },
            remove = { send(JSONObject().put("t", "removeFromGroup").put("id", cardTab)); open("switcher") },
            close = { closeTabs(listOf(cardTab)); if (sheet == "card") open("switcher") })
        return s
    }

    // What a long press on a link or an image offers.
    private fun linkSheetView(): View? {
        val m = pressed ?: return null
        val s = Sheet(context, "Link", palette) { open("") }
        linkSheet(s, palette, m, workspaces, open = { fields ->
            open("")
            fields.put("t", "newTab")
            send(fields)
        }, done = { open("") })
        return s
    }

    // MARK: Tabs

    // A new tab, or with a tab's id, one in that tab's group, starting one with it when it has none.
    private fun newTab(groupWith: Long) {
        open("")
        send(if (groupWith != 0L) JSONObject().put("t", "newTab").put("groupWith", groupWith)
             else JSONObject().put("t", "newTab").put("workspace", workspace))
    }

    private fun pick(id: Long) {
        open("")
        if (id != active) send(JSONObject().put("t", "activateTab").put("id", id)) else focusPage()
    }

    // The window never has no tabs: closing the last ones leaves a new one. Closing the active
    // tab of a group goes to its neighbour in the group.
    private fun closeTabs(ids: List<Long>) {
        val group = groups.firstOrNull { Switcher.inGroup(it, active) }
        if (ids.size >= tabs.size) {
            open("")
            send(JSONObject().put("t", "newTab").put("workspace", workspace))
        } else if (active in ids && group != null) {
            val members = Switcher.memberTabs(group, tabs).map { it.getLong("id") }
            val left = members.filter { it !in ids }
            val at = members.indexOf(active)
            left.getOrNull(minOf(at, left.size - 1))?.let { send(JSONObject().put("t", "activateTab").put("id", it)) }
        }
        for (id in ids) send(JSONObject().put("t", "closeTab").put("id", id))
    }

    // The tab before or after the active one: in its group while it is in one.
    private fun swipe(by: Int) {
        val group = groups.firstOrNull { Switcher.inGroup(it, active) }
        val ids = if (group != null) Switcher.memberTabs(group, tabs).map { it.getLong("id") } else tabs.map { it.getLong("id") }
        ids.getOrNull(ids.indexOf(active) + by)?.let { send(JSONObject().put("t", "activateTab").put("id", it)) }
    }

    // MARK: Editing the address

    // A tap turns the pill into the field, with the whole address selected.
    private fun begin() {
        if (editing) return
        editing = true
        if (sheet.isNotEmpty()) open("")
        send(JSONObject().put("t", "focusShell"))
        backClosesNow()
        address.setText(activeTab()?.optString("url") ?: "")
        // After the tap that focused it has placed the cursor: the whole address, as on the web.
        address.post { if (editing) address.selectAll() }
        draw()
        imm().showSoftInput(address, InputMethodManager.SHOW_IMPLICIT)
    }

    private fun end() {
        if (!editing) return
        editing = false
        asked++
        suggest(JSONArray())
        backClosesNow()
        imm().hideSoftInputFromWindow(windowToken, 0)
        draw()
    }

    // Typing asks Go for suggestions.
    private fun typed() {
        clear.visibility = if (address.text.isNotEmpty()) VISIBLE else GONE
        val text = address.text.toString()
        if (text.isBlank()) {
            asked++
            suggest(JSONArray())
        } else {
            send(JSONObject().put("t", "suggest").put("id", ++asked).put("text", text))
        }
    }

    // The suggestions, from the field outwards: over a field at the foot with the first
    // nearest it, under one on top. Each 48dp: an icon, the title, the host under it, and ↖,
    // which puts its address in the field to keep editing.
    private fun suggest(items: JSONArray) {
        suggestions.removeAllViews()
        val rows = (0 until items.length()).map { items.getJSONObject(it) }
        val ordered = if (addressBarPosition == "top") rows else rows.reversed()
        for (item in ordered) {
            val url = item.optString("url")
            val title = item.optString("title")
            val row = LinearLayout(context).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                setPadding(dp(16), 0, 0, 0)
                isClickable = true
                setOnClickListener { go(url) }
                background = android.graphics.drawable.StateListDrawable().apply {
                    addState(intArrayOf(android.R.attr.state_pressed), android.graphics.drawable.ColorDrawable(palette.press))
                }
            }
            asButton(row, "Go to ${title.ifEmpty { url }}")
            row.addView(IconView(context, if (title.isNotEmpty()) "globe" else "search", palette.icon), LinearLayout.LayoutParams(dp(24), dp(24)).apply { marginEnd = dp(16) })
            val lines = LinearLayout(context).apply { orientation = LinearLayout.VERTICAL }
            lines.addView(label(context, 14f, palette.text).apply { text = title.ifEmpty { url } })
            if (title.isNotEmpty()) lines.addView(label(context, 12f, palette.muted).apply { text = url.replace(Regex("^https?://(www\\.)?"), "") })
            row.addView(lines, LinearLayout.LayoutParams(0, LayoutParams.WRAP_CONTENT, 1f))
            val fill = IconButton(context, "fill", "Edit ${title.ifEmpty { url }}").apply {
                ink = palette.icon; press = palette.press
                setOnClickListener { address.setText(url); address.setSelection(url.length) }
            }
            row.addView(fill, LinearLayout.LayoutParams(dp(48), dp(48)))
            suggestions.addView(row, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, dp(48)))
        }
        suggestions.visibility = if (rows.isEmpty()) GONE else VISIBLE
        report()
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
        val w = (width / density).toDouble()
        val h = (height / density).toDouble()
        val px = heights()
        val topH = px.top / density.toDouble()
        val bottomH = px.bottom / density.toDouble()
        val top = addressBarPosition == "top"
        val shell = JSONArray()
        // A sheet covers the window, the switcher all but the bars; the bars are always the
        // shell's, and the suggestions and a page's notice while they show.
        if (sheet.isNotEmpty()) {
            shell.put(rect(0.0, 0.0, w, h))
        } else {
            if (topH > 0) shell.put(rect(0.0, 0.0, w, topH))
            if (bottomH > 0) shell.put(rect(0.0, h - bottomH, w, bottomH))
            if (suggestions.visibility == VISIBLE && suggestions.height > 0) {
                val sh = suggestions.height / density.toDouble()
                shell.put(rect(0.0, if (top) topH else h - bottomH - sh, w, sh))
            }
            if (notice.visibility == VISIBLE) shell.put(rect(0.0, topH, w, h - topH - bottomH))
        }
        val tabsShown = JSONArray()
        if (active != 0L) {
            tabsShown.put(JSONObject().put("tab", active).put("rect", rect(0.0, topH, w, h - topH - bottomH)))
        }
        val msg = JSONObject()
            .put("t", "layout")
            .put("shell", shell)
            .put("controls", JSONArray())
            .put("drag", JSONArray())
            .put("tabs", tabsShown)
            .put("viewW", w)
            .put("viewH", h)
            .put("frame", 0)
        val json = msg.toString()
        if (json != lastLayout) {
            lastLayout = json
            layoutShows = active
            sender.send(json)
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

    private fun send(msg: JSONObject) = sender.send(msg.toString())

    /** What the pill shows of an address at rest: its host, as the web shell's does. */
    private fun host(url: String): String {
        if (url.isEmpty()) return ""
        val u = android.net.Uri.parse(url)
        return if ((u.scheme == "http" || u.scheme == "https") && u.host != null) {
            u.host!!.removePrefix("www.") + if (u.port != -1) ":${u.port}" else ""
        } else {
            url
        }
    }

    /**
     * The address field. Empty, it says it is empty, as the web shell's field does, rather than
     * giving its hint as its text, as an EditText does; the hint stays its hint.
     */
    private class AddressField(context: Context) : EditText(context) {
        override fun onInitializeAccessibilityNodeInfo(info: android.view.accessibility.AccessibilityNodeInfo) {
            super.onInitializeAccessibilityNodeInfo(info)
            if (text.isEmpty()) {
                info.text = ""
                info.isShowingHintText = false
                info.hintText = hint
            }
        }
    }

    /** The bar: a swipe sideways along it goes to the tab before or after. */
    private inner class Toolbar(context: Context) : Bar(context) {
        private val slop = ViewConfiguration.get(context).scaledTouchSlop
        private var x0 = 0f
        private var y0 = 0f

        override fun onInterceptTouchEvent(e: MotionEvent): Boolean {
            when (e.actionMasked) {
                MotionEvent.ACTION_DOWN -> { x0 = e.x; y0 = e.y }
                MotionEvent.ACTION_MOVE -> if (!editing && abs(e.x - x0) > slop && abs(e.x - x0) > 2 * abs(e.y - y0)) return true
            }
            return false
        }

        @SuppressLint("ClickableViewAccessibility")
        override fun onTouchEvent(e: MotionEvent): Boolean {
            if (e.actionMasked == MotionEvent.ACTION_UP) {
                val dx = e.x - x0
                if (abs(dx) > dpf(48) && abs(dx) > 2 * abs(e.y - y0)) swipe(if (dx < 0) 1 else -1)
            }
            return true
        }
    }

    /** The tab count: a rounded square with the number of tabs, :D past 99, as Chrome has it. */
    private inner class CountButton(context: Context) : IconButton(context, "", "Tabs") {
        var count = 0
            set(v) { field = v; invalidate() }

        private val box = Paint(Paint.ANTI_ALIAS_FLAG).apply { style = Paint.Style.STROKE }
        private val digits = Paint(Paint.ANTI_ALIAS_FLAG).apply {
            textAlign = Paint.Align.CENTER
            typeface = Typeface.create(Typeface.DEFAULT, Typeface.BOLD)
        }

        override fun drawIcon(canvas: Canvas, color: Int) {
            val side = dpf(20)
            val cx = width / 2f
            val cy = height / 2f
            box.color = color
            box.strokeWidth = dpf(1.5)
            canvas.drawRoundRect(cx - side / 2, cy - side / 2, cx + side / 2, cy + side / 2, dpf(5), dpf(5), box)
            digits.color = color
            digits.textSize = dpf(11)
            canvas.drawText(if (count > 99) ":D" else count.toString(), cx, cy - (digits.descent() + digits.ascent()) / 2, digits)
        }
    }

    /** The root, to the driver: the native shell, as a page's WebView is to it a web one. */
    override fun getAccessibilityClassName(): CharSequence = "dev.projectb.browser.NativeShell"
}

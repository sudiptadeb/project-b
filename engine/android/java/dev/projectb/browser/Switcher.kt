package dev.projectb.browser

import android.annotation.SuppressLint
import android.content.Context
import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.drawable.GradientDrawable
import android.view.Gravity
import android.view.MotionEvent
import android.view.View
import android.view.ViewConfiguration
import android.view.accessibility.AccessibilityNodeInfo
import android.widget.FrameLayout
import android.widget.HorizontalScrollView
import android.widget.LinearLayout
import android.widget.ScrollView
import org.json.JSONObject
import kotlin.math.abs

/**
 * The tab switcher, over the page, as the web Mobile shell's Switcher.svelte: the workspaces as
 * chips while there are two or more, which choose the cards shown, and New Tab; then two columns
 * of cards, the active one ringed. A tab's card: a tap shows it, its × or a swipe sideways closes
 * it, a long press opens its menu. A group is one card where its first tab is, with its colour
 * along the top and its tabs' favicons, a tap on which goes to that tab; a tap elsewhere on it
 * opens the group's sheet.
 */
@SuppressLint("ViewConstructor")
class Switcher(context: Context, private val on: Actions) : FrameLayout(context) {
    interface Actions {
        fun pick(id: Long)
        fun closeTab(id: Long)
        fun newTab()
        fun card(id: Long)
        fun group(id: String)
        fun closeGroup(id: String)
        fun workspace(id: String)
    }

    private val head = LinearLayout(context)
    private val chips = LinearLayout(context)
    private val chipScroll = HorizontalScrollView(context)
    private val newTab = IconButton(context, "plus", "New Tab")
    private val grid = LinearLayout(context)
    private val scroll = ScrollView(context)

    init {
        isClickable = true
        contentDescription = "Tabs"
        head.orientation = LinearLayout.HORIZONTAL
        head.gravity = Gravity.CENTER_VERTICAL
        head.setPadding(dp(12), 0, dp(4), 0)
        chips.orientation = LinearLayout.HORIZONTAL
        chipScroll.isHorizontalScrollBarEnabled = false
        chipScroll.addView(chips)
        head.addView(chipScroll, LinearLayout.LayoutParams(0, LayoutParams.WRAP_CONTENT, 1f))
        newTab.setOnClickListener { on.newTab() }
        head.addView(newTab, LinearLayout.LayoutParams(dp(48), dp(48)))
        grid.orientation = LinearLayout.VERTICAL
        grid.setPadding(dp(12), 0, dp(12), dp(12))
        scroll.addView(grid)
        scroll.overScrollMode = OVER_SCROLL_NEVER
        val column = LinearLayout(context).apply { orientation = LinearLayout.VERTICAL }
        column.addView(head, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, dp(48)))
        column.addView(scroll, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, 0, 1f))
        addView(column)
    }

    /** Draws Go's state: the tabs, the active one, the groups, the workspaces, in the one shown. */
    fun show(palette: Palette, tabs: List<JSONObject>, active: Long, groups: List<JSONObject>,
             workspaces: List<JSONObject>, workspace: String) {
        setBackgroundColor(palette.bar)
        newTab.ink = palette.icon
        newTab.press = palette.press

        chips.removeAllViews()
        if (workspaces.size > 1) {
            for (w in workspaces) {
                val id = w.optString("id")
                val chosen = id == workspace
                val chip = LinearLayout(context).apply {
                    orientation = LinearLayout.HORIZONTAL
                    gravity = Gravity.CENTER_VERTICAL
                    setPadding(dp(12), 0, dp(12), 0)
                    background = if (chosen) rounded(palette.accentSoft, dpf(16))
                    else rounded(palette.surface, dpf(16), palette.line, dp(1))
                    isClickable = true
                    setOnClickListener { on.workspace(id) }
                    isSelected = chosen
                    contentDescription = w.optString("name")
                }
                val color = w.optString("color")
                if (color.isNotEmpty()) {
                    chip.addView(View(context).apply { background = GradientDrawable().apply { shape = GradientDrawable.OVAL; setColor(Palette.parseCss(color)) } },
                        LinearLayout.LayoutParams(dp(10), dp(10)).apply { marginEnd = dp(6) })
                }
                chip.addView(label(context, 13f, palette.text, bold = true).apply { text = w.optString("name") })
                chips.addView(chip, LinearLayout.LayoutParams(LayoutParams.WRAP_CONTENT, dp(48)).apply {
                    marginEnd = dp(8)
                    // The chip is 32dp high, in a 48dp target.
                    topMargin = dp(8); bottomMargin = dp(8)
                })
            }
        }

        // What the grid shows, in Go's order: a tab, or a group at its first tab.
        val items = ArrayList<View>()
        val done = HashSet<String>()
        for (tab in tabs) {
            val g = groups.firstOrNull { inGroup(it, tab.getLong("id")) }
            if (g == null) {
                if ((tab.optString("workspace")) == workspace) items.add(tabCard(palette, tab, active))
            } else if (done.add(g.getString("id"))) {
                val members = memberTabs(g, tabs)
                if ((members.firstOrNull()?.optString("workspace") ?: "") == workspace) items.add(groupCard(palette, g, members, active))
            }
        }
        grid.removeAllViews()
        var row: LinearLayout? = null
        for ((i, item) in items.withIndex()) {
            if (i % 2 == 0) {
                row = LinearLayout(context).apply { orientation = LinearLayout.HORIZONTAL }
                grid.addView(row, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT).apply { if (i > 0) topMargin = dp(12) })
            }
            row!!.addView(item, LinearLayout.LayoutParams(0, LayoutParams.WRAP_CONTENT, 1f).apply { if (i % 2 == 1) marginStart = dp(12) })
        }
        if (items.size % 2 == 1) row!!.addView(View(context), LinearLayout.LayoutParams(0, 1, 1f).apply { marginStart = dp(12) })
    }

    private fun card(palette: Palette, ring: Boolean, groupColor: Int?): LinearLayout =
        object : LinearLayout(context) {
            private val bar = Paint()
            override fun dispatchDraw(canvas: Canvas) {
                super.dispatchDraw(canvas)
                // A group's colour along the card's top, 4dp, inside its rounded corners.
                if (groupColor != null) {
                    bar.color = groupColor
                    canvas.save()
                    val clip = android.graphics.Path().apply { addRoundRect(0f, 0f, width.toFloat(), height.toFloat(), dpf(12), dpf(12), android.graphics.Path.Direction.CW) }
                    canvas.clipPath(clip)
                    canvas.drawRect(0f, 0f, width.toFloat(), dpf(4), bar)
                    canvas.restore()
                }
            }
        }.apply {
            orientation = LinearLayout.VERTICAL
            background = rounded(palette.surface, dpf(12), if (ring) palette.accent else palette.line, if (ring) dp(2) else dp(1))
            clipToOutline = true
            isClickable = true
        }

    private fun topRow(palette: Palette, title: String, closeName: String, onClose: () -> Unit): LinearLayout {
        val top = LinearLayout(context).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            setPadding(dp(12), 0, 0, 0)
        }
        top.addView(label(context, 13f, palette.text, bold = true).apply { text = title }, LinearLayout.LayoutParams(0, LayoutParams.WRAP_CONTENT, 1f))
        val close = IconButton(context, "close", closeName).apply {
            ink = palette.icon; press = palette.press; iconSize = 16f
            setOnClickListener { onClose() }
        }
        top.addView(close, LinearLayout.LayoutParams(dp(48), dp(48)))
        return top
    }

    private fun body(palette: Palette): FrameLayout = object : FrameLayout(context) {
        // 5:4, as the web card's body.
        override fun onMeasure(w: Int, h: Int) {
            val width = MeasureSpec.getSize(w)
            super.onMeasure(w, MeasureSpec.makeMeasureSpec(width * 4 / 5, MeasureSpec.EXACTLY))
        }
    }.apply { background = rounded(palette.accentTint, dpf(8)) }

    private fun host(palette: Palette, text: String) = label(context, 12f, palette.muted).apply {
        this.text = text
        gravity = Gravity.CENTER_VERTICAL
        setPadding(dp(12), 0, dp(12), 0)
    }

    private fun tabCard(palette: Palette, tab: JSONObject, active: Long): View {
        val id = tab.getLong("id")
        val name = tabLabel(tab)
        val c = card(palette, id == active, null)
        c.addView(topRow(palette, name, "Close $name") { on.closeTab(id) }, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, dp(40)))
        val b = body(palette)
        b.addView(FaviconView(context, palette, 32f).apply { set(tab) }, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT))
        c.addView(b, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT).apply { marginStart = dp(8); marginEnd = dp(8) })
        c.addView(host(palette, tab.optString("url").let { if (it.isEmpty()) "New tab" else hostOf(it) }), LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, dp(28)))
        // A tab, to a screen reader and the driver: its label, selected while active.
        c.contentDescription = name
        c.isSelected = id == active
        c.accessibilityDelegate = object : AccessibilityDelegate() {
            override fun onInitializeAccessibilityNodeInfo(host: View, info: AccessibilityNodeInfo) {
                super.onInitializeAccessibilityNodeInfo(host, info)
                info.className = "android.widget.TabWidget"
                info.isSelected = id == active
            }
        }
        c.setOnClickListener { on.pick(id) }
        c.setOnLongClickListener { on.card(id); true }
        swipeToClose(c) { on.closeTab(id) }
        return c
    }

    private fun groupCard(palette: Palette, g: JSONObject, members: List<JSONObject>, active: Long): View {
        val id = g.getString("id")
        val name = groupLabel(g)
        val c = card(palette, members.any { it.getLong("id") == active }, Palette.parseCss(g.optString("color", "#34404f")))
        c.addView(topRow(palette, name, "Close group $name") { on.closeGroup(id) }, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, dp(40)).apply { topMargin = dp(4) })
        val b = body(palette)
        // Up to four of its tabs' favicons, two by two; a tap on one goes to that tab.
        val cluster = LinearLayout(context).apply { orientation = LinearLayout.VERTICAL; setPadding(dp(8), dp(8), dp(8), dp(8)) }
        for (r in 0 until 2) {
            val line = LinearLayout(context).apply { orientation = LinearLayout.HORIZONTAL }
            for (k in 0 until 2) {
                val tab = members.getOrNull(r * 2 + k)
                val cell = FaviconView(context, palette, 28f)
                if (tab != null) {
                    cell.set(tab)
                    val tid = tab.getLong("id")
                    asButton(cell, tabLabel(tab))
                    cell.setOnClickListener { on.pick(tid) }
                    cell.setOnLongClickListener { on.card(tid); true }
                } else {
                    cell.visibility = INVISIBLE
                }
                line.addView(cell, LinearLayout.LayoutParams(0, LayoutParams.MATCH_PARENT, 1f))
            }
            cluster.addView(line, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, 0, 1f))
        }
        b.addView(cluster, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT))
        c.addView(b, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT).apply { marginStart = dp(8); marginEnd = dp(8) })
        c.addView(host(palette, if (g.optString("name").isNotEmpty()) "${members.size} tabs" else ""), LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, dp(28)))
        asButton(c, "Group $name")
        c.setOnClickListener { on.group(id) }
        return c
    }

    // A swipe sideways past a third of the card closes it; short of that it springs back.
    @SuppressLint("ClickableViewAccessibility")
    private fun swipeToClose(v: View, close: () -> Unit) {
        val slop = ViewConfiguration.get(context).scaledTouchSlop
        var x0 = 0f
        var y0 = 0f
        var sliding = false
        v.setOnTouchListener { view, e ->
            when (e.actionMasked) {
                MotionEvent.ACTION_DOWN -> { x0 = e.rawX; y0 = e.rawY; sliding = false }
                MotionEvent.ACTION_MOVE -> {
                    val dx = e.rawX - x0
                    if (!sliding && abs(dx) > slop && abs(dx) > 2 * abs(e.rawY - y0)) {
                        sliding = true
                        view.parent.requestDisallowInterceptTouchEvent(true)
                        view.cancelLongPress()
                        view.isPressed = false
                    }
                    if (sliding) {
                        view.translationX = dx
                        view.alpha = 1f - minOf(1f, abs(dx) / view.width)
                    }
                }
                MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> if (sliding) {
                    val dx = e.rawX - x0
                    if (e.actionMasked == MotionEvent.ACTION_UP && abs(dx) > view.width / 3f) {
                        view.animate().translationX(if (dx > 0) view.width * 1.2f else -view.width * 1.2f).alpha(0f).setDuration(120).withEndAction { close() }.start()
                    } else {
                        view.animate().translationX(0f).alpha(1f).setDuration(150).start()
                    }
                    sliding = false
                    return@setOnTouchListener true
                }
            }
            sliding
        }
    }

    companion object {
        fun inGroup(g: JSONObject, tab: Long): Boolean {
            val ids = g.optJSONArray("tabs") ?: return false
            for (i in 0 until ids.length()) if (ids.getLong(i) == tab) return true
            return false
        }

        fun memberTabs(g: JSONObject, tabs: List<JSONObject>): List<JSONObject> {
            val ids = g.optJSONArray("tabs") ?: return emptyList()
            return (0 until ids.length()).mapNotNull { i -> tabs.firstOrNull { it.getLong("id") == ids.getLong(i) } }
        }

        /** What a group is called: its name, else how many tabs it has. */
        fun groupLabel(g: JSONObject): String =
            g.optString("name").ifEmpty { "${g.optJSONArray("tabs")?.length() ?: 0} tabs" }
    }
}

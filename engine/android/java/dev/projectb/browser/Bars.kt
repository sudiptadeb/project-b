package dev.projectb.browser

import android.annotation.SuppressLint
import android.content.Context
import android.graphics.Canvas
import android.graphics.Paint
import android.view.Gravity
import android.widget.FrameLayout
import android.widget.HorizontalScrollView
import android.widget.LinearLayout
import android.widget.TextView
import org.json.JSONObject

/**
 * A bar of the shell, its hairline on its top edge, or its bottom edge for the address bar on
 * top: 1dp of --line, inside the bar's own height.
 */
open class Bar(context: Context) : LinearLayout(context) {
    var lineOnTop = true
        set(v) { field = v; invalidate() }
    var line = 0
        set(v) { field = v; invalidate() }
    private val paint = Paint()

    init {
        setWillNotDraw(false)
        orientation = HORIZONTAL
        gravity = Gravity.CENTER_VERTICAL
    }

    override fun dispatchDraw(canvas: Canvas) {
        super.dispatchDraw(canvas)
        paint.color = line
        val y = if (lineOnTop) 0f else height - 1f
        canvas.drawRect(0f, y, width.toFloat(), y + 1f, paint)
    }
}

/**
 * The group strip, while the active tab is in a group, as the web shell's Strip.svelte: the
 * group's dot, which opens its sheet; its tabs as favicons, the active one on a pill with an ×
 * on it; and + for a new tab in the group. A tap on a favicon goes to its tab, a tap on the
 * active one closes it, and a long press closes any. Every target 48dp; the strip is 48 and its
 * hairline.
 */
@SuppressLint("ViewConstructor")
class Strip(context: Context, private val on: Actions) : Bar(context) {
    interface Actions {
        fun groupSheet()
        fun pick(id: Long)
        fun closeTab(id: Long)
        fun newTabInGroup()
    }

    private val dot = object : IconButton(context, "", "Group") {
        var color = 0
        override fun drawIcon(canvas: Canvas, color: Int) {
            val p = Paint(Paint.ANTI_ALIAS_FLAG)
            p.color = this.color
            canvas.drawCircle(width / 2f, height / 2f, dpf(5), p)
        }
    }
    private val row = LinearLayout(context)
    private val scroll = HorizontalScrollView(context)

    init {
        setPadding(dp(4), 1, dp(4), 0)
        dot.setOnClickListener { on.groupSheet() }
        addView(dot, LayoutParams(dp(48), dp(48)))
        row.orientation = HORIZONTAL
        scroll.isHorizontalScrollBarEnabled = false
        scroll.addView(row)
        addView(scroll, LayoutParams(0, dp(48), 1f))
    }

    fun show(palette: Palette, group: JSONObject, members: List<JSONObject>, active: Long) {
        setBackgroundColor(palette.bar)
        line = palette.line
        val name = Switcher.groupLabel(group)
        dot.color = Palette.parseCss(group.optString("color", "#34404f"))
        dot.press = palette.press
        dot.contentDescription = "Group $name"
        dot.invalidate()
        contentDescription = "Group $name"
        row.removeAllViews()
        var activeView: android.view.View? = null
        for (tab in members) {
            val id = tab.getLong("id")
            val on = id == active
            val b = FaviconButton(context, palette, tab, on)
            asButton(b, if (on) "Close ${tabLabel(tab)}" else "Go to ${tabLabel(tab)}")
            b.isSelected = on
            b.setOnClickListener { if (on) this.on.closeTab(id) else this.on.pick(id) }
            b.setOnLongClickListener { this.on.closeTab(id); true }
            row.addView(b, LayoutParams(dp(48), dp(48)))
            if (on) activeView = b
        }
        row.addView(IconButton(context, "plus", "New tab in group").apply {
            ink = palette.icon; press = palette.press
            setOnClickListener { on.newTabInGroup() }
        }, LayoutParams(dp(48), dp(48)))
        // The active tab scrolled into view.
        activeView?.let { v -> scroll.post { scroll.requestChildRectangleOnScreen(row, android.graphics.Rect(v.left, 0, v.right, v.height), false) } }
    }

    /** A tab's favicon, 20dp; the active one on the surface's pill, an × at its corner. */
    private class FaviconButton(context: Context, private val palette: Palette, tab: JSONObject, private val on: Boolean) :
        FrameLayout(context) {
        private val paint = Paint(Paint.ANTI_ALIAS_FLAG)

        init {
            setWillNotDraw(false)
            isClickable = true
            addView(FaviconView(context, palette, 20f).apply { set(tab) }, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT))
        }

        override fun onDraw(canvas: Canvas) {
            if (on) {
                paint.style = Paint.Style.FILL
                paint.color = palette.surface
                canvas.drawRoundRect(dpf(4), dpf(4), width - dpf(4), height - dpf(4), dpf(12), dpf(12), paint)
                paint.style = Paint.Style.STROKE
                paint.strokeWidth = 1f
                paint.color = palette.line
                canvas.drawRoundRect(dpf(4), dpf(4), width - dpf(4), height - dpf(4), dpf(12), dpf(12), paint)
            } else if (isPressed) {
                paint.style = Paint.Style.FILL
                paint.color = palette.press
                canvas.drawRoundRect(dpf(4), dpf(4), width - dpf(4), height - dpf(4), dpf(12), dpf(12), paint)
            }
        }

        override fun dispatchDraw(canvas: Canvas) {
            super.dispatchDraw(canvas)
            if (on) Icons.draw(canvas, "close", palette.icon, dpf(10), width - dpf(9), dpf(9))
        }

        override fun drawableStateChanged() {
            super.drawableStateChanged()
            invalidate()
        }
    }
}

/** The last run's crash, in a line over the toolbar until it is dismissed. */
class CrashBar(context: Context, private val dismiss: () -> Unit) : Bar(context) {
    private val text = label(context, 12f, 0).apply { maxLines = 2; setSingleLine(false) }
    private val button = TextView(context)

    init {
        setPadding(dp(16), 1, dp(4), 0)
        addView(text, LayoutParams(0, LayoutParams.WRAP_CONTENT, 1f))
        button.text = "Dismiss"
        button.gravity = Gravity.CENTER
        button.setPadding(dp(16), 0, dp(16), 0)
        button.isClickable = true
        button.setOnClickListener { dismiss() }
        asButton(button, "Dismiss")
        addView(button, LayoutParams(LayoutParams.WRAP_CONTENT, dp(48)))
    }

    fun show(palette: Palette, log: String) {
        setBackgroundColor(palette.bar)
        line = palette.line
        text.setTextColor(palette.text)
        text.text = "Project B crashed last time. The log is at $log"
        button.setTextColor(palette.muted)
    }
}

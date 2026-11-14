package dev.projectb.browser

import android.animation.ValueAnimator
import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.net.Uri
import android.provider.Settings
import android.text.TextUtils
import android.util.Base64
import android.util.TypedValue
import android.view.Gravity
import android.view.MotionEvent
import android.view.View
import android.view.ViewGroup
import android.view.accessibility.AccessibilityNodeInfo
import android.view.animation.DecelerateInterpolator
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.TextView
import org.json.JSONObject

// The native Mobile shell's building blocks, each the web Mobile shell's in size and colour
// (docs/shells.md, "The mobile shell"; the spec's section 7).

/** The design system's tokens, from one of Go's palettes (core/tones), by name. */
class Palette(json: JSONObject?) {
    private val tokens = HashMap<String, Int>()

    init {
        json?.keys()?.forEach { tokens[it] = parseCss(json.getString(it)) }
    }

    operator fun get(name: String): Int = tokens[name] ?: Color.TRANSPARENT

    val bar get() = this["--bar"]
    val surface get() = this["--surface"]
    val text get() = this["--text"]
    val muted get() = this["--muted"]
    val icon get() = this["--icon"]
    val line get() = this["--line"]
    val press get() = this["--press"]
    val accent get() = this["--accent"]
    val onAccent get() = this["--on-accent"]
    val accentSoft get() = this["--accent-soft"]
    val accentTint get() = this["--accent-tint"]
    val danger get() = this["--danger"]

    companion object {
        /** #rrggbb or rgba(r, g, b, a), as the tones are written. */
        fun parseCss(css: String): Int {
            if (css.startsWith("#")) return Color.parseColor(css)
            val m = Regex("""rgba\(\s*(\d+),\s*(\d+),\s*(\d+),\s*([\d.]+)\s*\)""").find(css) ?: return Color.TRANSPARENT
            val (r, g, b, a) = m.destructured
            return Color.argb((a.toFloat() * 255 + 0.5f).toInt(), r.toInt(), g.toInt(), b.toInt())
        }

        /** A colour at a fraction of its alpha, as color-mix(in srgb, c 70%, transparent). */
        fun alpha(c: Int, f: Float) = Color.argb((Color.alpha(c) * f).toInt(), Color.red(c), Color.green(c), Color.blue(c))
    }
}

fun View.dp(v: Number): Int = (v.toFloat() * resources.displayMetrics.density + 0.5f).toInt()

fun View.dpf(v: Number): Float = v.toFloat() * resources.displayMetrics.density

/** Whether motion is reduced, as prefers-reduced-motion says on the web: animations off. */
fun reducedMotion(context: Context): Boolean =
    Settings.Global.getFloat(context.contentResolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f) == 0f

fun rounded(color: Int, radius: Float, stroke: Int = Color.TRANSPARENT, strokeWidth: Int = 0) =
    GradientDrawable().apply {
        setColor(color)
        cornerRadius = radius
        if (strokeWidth > 0) setStroke(strokeWidth, stroke)
    }

/** A text view in the design system's type: size in sp-as-dp, as the web's px. */
fun label(context: Context, size: Float, color: Int, bold: Boolean = false) = TextView(context).apply {
    setTextSize(TypedValue.COMPLEX_UNIT_DIP, size)
    setTextColor(color)
    if (bold) typeface = Typeface.create("sans-serif-medium", Typeface.NORMAL)
    setSingleLine(true)
    ellipsize = TextUtils.TruncateAt.END
    includeFontPadding = false
}

/** Says a view is a button to a screen reader and the driver, named label. */
fun asButton(v: View, name: String) {
    v.contentDescription = name
    v.accessibilityDelegate = object : View.AccessibilityDelegate() {
        override fun onInitializeAccessibilityNodeInfo(host: View, info: AccessibilityNodeInfo) {
            super.onInitializeAccessibilityNodeInfo(host, info)
            info.className = "android.widget.Button"
        }
    }
}

/**
 * A round icon button, 48dp to touch, its icon 20dp in the colour given, and a 40dp circle of
 * the press colour behind it while held, as the web shell's 40px buttons in a 48px target.
 */
open class IconButton(context: Context, var icon: String, name: String) : View(context) {
    var ink = Color.DKGRAY
        set(v) { field = v; invalidate() }
    var press = Color.TRANSPARENT
    var iconSize = 20f

    private val behind = Paint(Paint.ANTI_ALIAS_FLAG)

    init {
        isClickable = true
        asButton(this, name)
    }

    override fun onDraw(canvas: Canvas) {
        if (isPressed) {
            behind.color = press
            canvas.drawCircle(width / 2f, height / 2f, dpf(20), behind)
        }
        val c = if (isEnabled) ink else Palette.alpha(ink, 0.38f)
        drawIcon(canvas, c)
    }

    open fun drawIcon(canvas: Canvas, color: Int) =
        Icons.draw(canvas, icon, color, dpf(iconSize), width / 2f, height / 2f)

    override fun drawableStateChanged() {
        super.drawableStateChanged()
        invalidate()
    }
}

/**
 * A bottom sheet over the window: the backdrop, the bar's colour at 70%, a tap on which closes
 * it, and the panel, the surface with a 16dp top radius and a 32 by 4 handle, which slides up in
 * 200ms and which a swipe down on the handle closes. As the web shell's Sheet.svelte.
 */
class Sheet(context: Context, name: String, palette: Palette, private val onClose: () -> Unit) : FrameLayout(context) {
    val content = LinearLayout(context).apply { orientation = LinearLayout.VERTICAL }
    private val panel = LinearLayout(context).apply { orientation = LinearLayout.VERTICAL }

    init {
        setBackgroundColor(Palette.alpha(palette.bar, 0.7f))
        setOnClickListener { onClose() }
        contentDescription = name
        panel.background = GradientDrawable().apply {
            setColor(palette.surface)
            cornerRadii = floatArrayOf(dpf(16), dpf(16), dpf(16), dpf(16), 0f, 0f, 0f, 0f)
        }
        panel.elevation = dpf(8)
        panel.isClickable = true
        panel.setPadding(0, 0, 0, dp(8))
        val handle = object : FrameLayout(context) {
            var start = -1f
            override fun onTouchEvent(e: MotionEvent): Boolean {
                when (e.actionMasked) {
                    MotionEvent.ACTION_DOWN -> start = e.rawY
                    MotionEvent.ACTION_UP -> {
                        if (start >= 0 && e.rawY - start > dpf(40)) onClose()
                        start = -1f
                    }
                    MotionEvent.ACTION_CANCEL -> start = -1f
                }
                return true
            }
        }
        handle.addView(View(context).apply { background = rounded(palette.line, dpf(2)) },
            LayoutParams(dp(32), dp(4), Gravity.CENTER))
        panel.addView(handle, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, dp(20)))
        panel.addView(content, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT))
        addView(panel, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT, Gravity.BOTTOM))
    }

    override fun onAttachedToWindow() {
        super.onAttachedToWindow()
        if (reducedMotion(context)) return
        panel.translationY = 10000f
        panel.post {
            ValueAnimator.ofFloat(panel.height.toFloat(), 0f).apply {
                duration = 200
                interpolator = DecelerateInterpolator()
                addUpdateListener { panel.translationY = it.animatedValue as Float }
                start()
            }
        }
    }
}

/** A 48dp row of a sheet: an icon, its name, and a chevron or a tick at its end. */
fun sheetRow(context: Context, palette: Palette, icon: String?, name: String, end: String? = null,
             nameColor: Int = palette.text, onClick: () -> Unit): View {
    val row = LinearLayout(context)
    row.orientation = LinearLayout.HORIZONTAL
    row.gravity = Gravity.CENTER_VERTICAL
    row.setPadding(row.dp(16), 0, row.dp(12), 0)
    row.background = android.graphics.drawable.StateListDrawable().apply {
        addState(intArrayOf(android.R.attr.state_pressed), android.graphics.drawable.ColorDrawable(palette.press))
    }
    row.isClickable = true
    row.setOnClickListener { onClick() }
    asButton(row, name)
    if (icon != null) {
        row.addView(IconView(context, icon, palette.icon), LinearLayout.LayoutParams(row.dp(24), row.dp(24)).apply { marginEnd = row.dp(16) })
    }
    row.addView(label(context, 14f, nameColor), LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f))
    (row.getChildAt(row.childCount - 1) as TextView).text = name
    if (end != null) row.addView(IconView(context, end, palette.icon), LinearLayout.LayoutParams(row.dp(24), row.dp(24)))
    row.minimumHeight = row.dp(48)
    return row
}

/** An icon alone, drawn at 20dp in its box, or the size given. */
class IconView(context: Context, icon: String, color: Int, private val size: Float = 20f) : View(context) {
    var icon = icon
        set(v) { field = v; invalidate() }
    var color = color
        set(v) { field = v; invalidate() }

    override fun onDraw(canvas: Canvas) {
        if (icon.isNotEmpty()) Icons.draw(canvas, icon, color, dpf(size), width / 2f, height / 2f)
    }
}

/**
 * A tab's favicon, or the first letter of its label on the accent's soft fill while it has none,
 * as the web shell's Favicon.svelte.
 */
class FaviconView(context: Context, private val palette: Palette, private val sizeDp: Float) : View(context) {
    private var bitmap: Bitmap? = null
    private var letter = ""
    private val paint = Paint(Paint.ANTI_ALIAS_FLAG or Paint.FILTER_BITMAP_FLAG)

    fun set(tab: JSONObject) {
        bitmap = Favicons.of(tab.optString("favicon"))
        letter = tabLabel(tab).take(1).uppercase()
        invalidate()
    }

    override fun onDraw(canvas: Canvas) {
        val s = dpf(sizeDp)
        val l = (width - s) / 2
        val t = (height - s) / 2
        val b = bitmap
        if (b != null) {
            canvas.drawBitmap(b, null, android.graphics.RectF(l, t, l + s, t + s), paint)
            return
        }
        paint.color = palette.accentSoft
        canvas.drawRoundRect(l, t, l + s, t + s, dpf(4), dpf(4), paint)
        paint.color = palette.text
        paint.textAlign = Paint.Align.CENTER
        paint.textSize = s * 0.6f
        paint.typeface = Typeface.DEFAULT_BOLD
        canvas.drawText(letter, width / 2f, height / 2f - (paint.descent() + paint.ascent()) / 2, paint)
    }
}

/** Favicons from the data URLs Go sends, decoded once each. */
object Favicons {
    private val cache = object : android.util.LruCache<String, Bitmap>(64) {}

    fun of(url: String): Bitmap? {
        if (!url.startsWith("data:")) return null
        cache.get(url)?.let { return it }
        val comma = url.indexOf(',')
        if (comma < 0 || !url.substring(0, comma).endsWith(";base64")) return null
        val bytes = try { Base64.decode(url.substring(comma + 1), Base64.DEFAULT) } catch (e: IllegalArgumentException) { return null }
        val b = BitmapFactory.decodeByteArray(bytes, 0, bytes.size) ?: return null
        cache.put(url, b)
        return b
    }
}

/** What a tab is called: its title, else its address's host, else New Tab, as the SDK's tabLabel. */
fun tabLabel(tab: JSONObject): String {
    val title = tab.optString("title")
    if (title.isNotEmpty()) return title
    val url = tab.optString("url")
    if (url.isNotEmpty()) return Uri.parse(url).host?.takeIf { it.isNotEmpty() } ?: url
    return "New Tab"
}

/** A tab's address as the switcher's cards show it: its host, without www. */
fun hostOf(url: String): String {
    if (url.isEmpty()) return ""
    return Uri.parse(url).host?.removePrefix("www.")?.takeIf { it.isNotEmpty() } ?: url
}

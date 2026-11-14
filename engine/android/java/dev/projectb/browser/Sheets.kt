package dev.projectb.browser

import android.app.AlertDialog
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.graphics.drawable.GradientDrawable
import android.text.InputType
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.view.inputmethod.EditorInfo
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.TextView
import org.json.JSONObject

// The native Mobile shell's sheets, each the web Mobile shell's (GroupSheet, CardMenu,
// LinkSheet), drawn into a Sheet's content.

/** The colours a group can take, Go's GroupColors: the workspaces' seven. */
val groupColors = listOf("#3470d0", "#d03434", "#4f9e9e", "#e8a317", "#8a4fd0", "#2e9d5b", "#34404f")

private fun dot(context: Context, color: String, size: Int) = View(context).apply {
    background = GradientDrawable().apply { shape = GradientDrawable.OVAL; setColor(Palette.parseCss(color)) }
    minimumWidth = size
}

private fun head(context: Context, palette: Palette, text: String, sub: String? = null): View {
    val box = LinearLayout(context).apply { orientation = LinearLayout.VERTICAL }
    box.setPadding(box.dp(20), 0, box.dp(20), box.dp(8))
    if (text.isNotEmpty()) box.addView(label(context, 13f, palette.text, bold = true).apply { this.text = text })
    if (sub != null) box.addView(label(context, 12f, palette.muted).apply { this.text = sub })
    return box
}

/**
 * A group's sheet: its name, saved on Enter or on leaving the field; its colour; its tabs, a tap
 * on one going there, its × closing it; then New tab in it, Ungroup, and Close group, which asks
 * first.
 */
fun groupSheet(s: Sheet, palette: Palette, group: JSONObject, members: List<JSONObject>, ask: Boolean,
               change: (JSONObject) -> Unit, pick: (Long) -> Unit, closeTab: (Long) -> Unit,
               newTab: () -> Unit, ungroup: () -> Unit, closeGroup: () -> Unit) {
    val context = s.context
    val c = s.content
    val row = LinearLayout(context).apply { orientation = LinearLayout.HORIZONTAL; gravity = Gravity.CENTER_VERTICAL }
    row.setPadding(row.dp(20), row.dp(4), row.dp(16), row.dp(8))
    row.addView(dot(context, group.optString("color"), row.dp(10)), LinearLayout.LayoutParams(row.dp(10), row.dp(10)).apply { marginEnd = row.dp(14) })
    val name = EditText(context).apply {
        setText(group.optString("name"))
        hint = "Name"
        contentDescription = "Group name"
        setSingleLine(true)
        setTextSize(TypedValue.COMPLEX_UNIT_DIP, 14f)
        setTextColor(palette.text)
        setHintTextColor(palette.muted)
        inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_FLAG_CAP_SENTENCES
        imeOptions = EditorInfo.IME_ACTION_DONE
        background = rounded(palette.surface, dpf(8), palette.line, dp(1))
        setPadding(dp(12), 0, dp(12), 0)
        fun save() {
            val now = text.toString().trim()
            if (now != group.optString("name")) change(JSONObject().put("name", now))
        }
        setOnFocusChangeListener { _, has -> if (!has) save() }
        setOnEditorActionListener { v, _, _ -> save(); v.clearFocus(); false }
    }
    row.addView(name, LinearLayout.LayoutParams(0, row.dp(48), 1f))
    c.addView(row)

    val swatches = LinearLayout(context).apply { orientation = LinearLayout.HORIZONTAL; gravity = Gravity.CENTER_VERTICAL }
    swatches.setPadding(swatches.dp(12), 0, swatches.dp(12), swatches.dp(8))
    for (color in groupColors) {
        val chosen = group.optString("color") == color
        val sw = object : View(context) {
            private val paint = android.graphics.Paint(android.graphics.Paint.ANTI_ALIAS_FLAG)
            override fun onDraw(canvas: android.graphics.Canvas) {
                paint.style = android.graphics.Paint.Style.FILL
                paint.color = Palette.parseCss(color)
                canvas.drawCircle(width / 2f, height / 2f, dpf(13), paint)
                if (chosen) {
                    paint.style = android.graphics.Paint.Style.STROKE
                    paint.strokeWidth = dpf(2)
                    paint.color = palette.text
                    canvas.drawCircle(width / 2f, height / 2f, dpf(17), paint)
                }
            }
        }
        sw.isClickable = true
        sw.isSelected = chosen
        sw.contentDescription = color
        sw.setOnClickListener { change(JSONObject().put("color", color)) }
        swatches.addView(sw, LinearLayout.LayoutParams(0, swatches.dp(48), 1f))
    }
    c.addView(swatches)

    for (tab in members) {
        val id = tab.getLong("id")
        val r = LinearLayout(context).apply { orientation = LinearLayout.HORIZONTAL; gravity = Gravity.CENTER_VERTICAL }
        r.setPadding(r.dp(16), 0, 0, 0)
        val go = LinearLayout(context).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            isClickable = true
            setOnClickListener { pick(id) }
        }
        go.addView(FaviconView(context, palette, 20f).apply { set(tab) }, LinearLayout.LayoutParams(go.dp(24), go.dp(24)).apply { marginEnd = go.dp(16) })
        go.addView(label(context, 14f, palette.text).apply { text = tabLabel(tab) }, LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f))
        asButton(go, tabLabel(tab))
        r.addView(go, LinearLayout.LayoutParams(0, r.dp(48), 1f))
        r.addView(IconButton(context, "close", "Close ${tabLabel(tab)}").apply {
            ink = palette.icon; press = palette.press; iconSize = 16f
            setOnClickListener { closeTab(id) }
        }, LinearLayout.LayoutParams(r.dp(48), r.dp(48)))
        c.addView(r)
    }

    val answers = LinearLayout(context).apply { orientation = LinearLayout.HORIZONTAL; gravity = Gravity.END }
    answers.setPadding(answers.dp(12), answers.dp(8), answers.dp(12), 0)
    fun answer(text: String, color: Int, run: () -> Unit) {
        val b = label(context, 14f, color, bold = true).apply {
            this.text = text
            gravity = Gravity.CENTER
            setPadding(dp(16), 0, dp(16), 0)
            isClickable = true
            setOnClickListener { run() }
            background = android.graphics.drawable.StateListDrawable().apply {
                addState(intArrayOf(android.R.attr.state_pressed), rounded(palette.press, dpf(8)))
            }
        }
        asButton(b, text)
        answers.addView(b, LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT, answers.dp(48)))
    }
    fun confirm() {
        AlertDialog.Builder(context)
            .setTitle("Close ${members.size} tabs?")
            .setNegativeButton("Cancel", null)
            .setPositiveButton("Close") { _, _ -> closeGroup() }
            .show()
    }
    answer("New tab", palette.text, newTab)
    answer("Ungroup", palette.text, ungroup)
    answer("Close group…", palette.danger) { confirm() }
    c.addView(answers)
    if (ask) s.post { confirm() }
}

/**
 * A card's menu, from a long press on it in the switcher: Add to group, which lists the groups
 * of its workspace and New group, Remove from group while it is in one, and Close.
 */
fun cardMenu(s: Sheet, palette: Palette, tab: JSONObject, inGroup: Boolean, groups: List<JSONObject>,
             add: (String) -> Unit, newGroup: () -> Unit, remove: () -> Unit, close: () -> Unit) {
    val context = s.context
    val c = s.content
    c.addView(head(context, palette, tabLabel(tab)))
    c.addView(sheetRow(context, palette, "group", "Add to group", "chevron") {
        c.removeAllViews()
        c.addView(head(context, palette, tabLabel(tab)))
        for (g in groups) {
            c.addView(sheetRow(context, palette, null, Switcher.groupLabel(g)) { add(g.getString("id")) }.also { row ->
                (row as LinearLayout).addView(dot(context, g.optString("color"), row.dp(10)), 0,
                    LinearLayout.LayoutParams(row.dp(10), row.dp(10)).apply { marginEnd = row.dp(16) })
            })
        }
        c.addView(sheetRow(context, palette, "plus", "New group") { newGroup() })
    })
    if (inGroup) c.addView(sheetRow(context, palette, "tab", "Remove from group") { remove() })
    c.addView(sheetRow(context, palette, "close", "Close") { close() })
}

/**
 * What a long press on a link or an image in the page offers, drawn by the shell since a phone
 * has no menu of its own: for a link, Open in new tab in group first, as Chrome has it, then
 * Open in new tab, Open in workspace, and Copy link; for an image, Open image in new tab and Copy
 * image address. A tab opened here opens behind, so the group strip gathers them.
 */
fun linkSheet(s: Sheet, palette: Palette, menu: JSONObject, workspaces: List<JSONObject>,
              open: (JSONObject) -> Unit, done: () -> Unit) {
    val context = s.context
    val c = s.content
    val link = menu.optString("link")
    val url = link.ifEmpty { menu.optString("src") }
    val shown = url.replace(Regex("^https?://(www\\.)?"), "")
    c.addView(head(context, palette, if (link.isNotEmpty()) menu.optString("linkText") else "", shown))
    fun copy() {
        val clip = context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        clip.setPrimaryClip(ClipData.newPlainText("Link", url))
        done()
    }
    if (link.isNotEmpty()) {
        c.addView(sheetRow(context, palette, "group", "Open in new tab in group") {
            open(JSONObject().put("text", url).put("groupWith", menu.optLong("tab")).put("background", true))
        })
        c.addView(sheetRow(context, palette, "tab", "Open in new tab") { open(JSONObject().put("text", url).put("background", true)) })
        if (workspaces.size > 1) {
            c.addView(sheetRow(context, palette, "workspace", "Open in workspace", "chevron") {
                c.removeAllViews()
                c.addView(head(context, palette, "", shown))
                for (w in workspaces) {
                    c.addView(sheetRow(context, palette, null, w.optString("name")) {
                        open(JSONObject().put("text", url).put("workspace", w.optString("id")).put("background", true))
                    }.also { row ->
                        (row as LinearLayout).addView(dot(context, w.optString("color", "#9aa0a6"), row.dp(10)), 0,
                            LinearLayout.LayoutParams(row.dp(10), row.dp(10)).apply { marginEnd = row.dp(16) })
                    })
                }
            })
        }
        c.addView(sheetRow(context, palette, "copy", "Copy link") { copy() })
    } else {
        c.addView(sheetRow(context, palette, "image", "Open image in new tab") { open(JSONObject().put("text", url).put("background", true)) })
        c.addView(sheetRow(context, palette, "copy", "Copy image address") { copy() })
    }
}

/**
 * Over a page that crashed, failed to load or was blocked, which otherwise shows blank: one
 * plain sentence, the error's own name under it, and Try again or Reload.
 */
class PageNotice(context: Context, private val again: () -> Unit) : LinearLayout(context) {
    private val mark = IconView(context, "warn", 0, 32f)
    private val said = TextView(context)
    private val code = TextView(context)
    private val button = TextView(context)

    init {
        orientation = VERTICAL
        gravity = Gravity.CENTER
        isClickable = true
        setPadding(dp(24), 0, dp(24), 0)
        addView(mark, LayoutParams(dp(48), dp(48)))
        said.setTextSize(TypedValue.COMPLEX_UNIT_DIP, 16f)
        said.gravity = Gravity.CENTER
        addView(said, LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.WRAP_CONTENT).apply { topMargin = dp(12) })
        code.setTextSize(TypedValue.COMPLEX_UNIT_DIP, 12f)
        addView(code, LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.WRAP_CONTENT).apply { topMargin = dp(4) })
        button.setTextSize(TypedValue.COMPLEX_UNIT_DIP, 14f)
        button.gravity = Gravity.CENTER
        button.setPadding(dp(20), 0, dp(20), 0)
        button.isClickable = true
        button.setOnClickListener { again() }
        addView(button, LayoutParams(LayoutParams.WRAP_CONTENT, dp(48)).apply { topMargin = dp(20) })
    }

    fun show(palette: Palette, tab: JSONObject) {
        setBackgroundColor(palette.bar)
        val host = hostOf(tab.optString("url"))
        val err = tab.optString("error").removePrefix("net::")
        val crashed = tab.optBoolean("crashed")
        val blocked = tab.optBoolean("blocked")
        mark.icon = if (crashed) "reload" else "warn"
        mark.color = palette.icon
        said.setTextColor(palette.text)
        said.text = when {
            blocked -> "$host is blocked"
            crashed -> "This page crashed"
            Regex("NAME_NOT_RESOLVED|NAME_RESOLUTION_FAILED").containsMatchIn(err) -> "Can't find $host"
            err.contains("INTERNET_DISCONNECTED") -> "No internet connection"
            else -> "Can't reach $host"
        }
        code.setTextColor(palette.muted)
        code.text = err
        code.visibility = if (err.isNotEmpty() && !crashed && !blocked) VISIBLE else GONE
        button.text = if (crashed) "Reload" else "Try again"
        button.setTextColor(palette.onAccent)
        button.background = rounded(palette.accent, dpf(8))
        asButton(button, button.text.toString())
        button.visibility = if (blocked) GONE else VISIBLE
    }
}

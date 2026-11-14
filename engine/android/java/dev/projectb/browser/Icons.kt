package dev.projectb.browser

import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.Path
import android.graphics.RectF
import kotlin.math.PI
import kotlin.math.abs
import kotlin.math.acos
import kotlin.math.sqrt

/**
 * The native Mobile shell's line icons: the web Mobile shell's own (its Icon.svelte), the same
 * SVG path data on a 24-unit box, drawn at a 1.5-unit round stroke in the colour around them,
 * so the two shells show the same icons.
 */
object Icons {
    val paths = mapOf(
        "back" to "M15 5l-7 7 7 7",
        "forward" to "M9 5l7 7-7 7",
        "reload" to "M19 12a7 7 0 1 1-2.05-4.95M19 4.5V8h-3.5",
        "stop" to "M6 6l12 12M18 6L6 18",
        "plus" to "M12 5v14M5 12h14",
        "close" to "M7 7l10 10M17 7L7 17",
        "lock" to "M7.5 11V8.5a4.5 4.5 0 0 1 9 0V11M6.5 11h11v8h-11z",
        "warn" to "M12 4l8.5 15h-17zM12 10v4M12 16.8h.01",
        "search" to "M10.5 17a6.5 6.5 0 1 0 0-13 6.5 6.5 0 0 0 0 13zM15.5 15.5L20 20",
        "globe" to "M12 20a8 8 0 1 0 0-16 8 8 0 0 0 0 16zM4 12h16M12 4c2.2 2.2 3.2 5 3.2 8s-1 5.8-3.2 8c-2.2-2.2-3.2-5-3.2-8s1-5.8 3.2-8z",
        "fill" to "M7 7h7M7 7v7M7 7l10 10",
        "settings" to "M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM19.4 13.5l1.6 1.2-2 3.4-1.9-.7a7 7 0 0 1-2 1.2L14.8 21h-4l-.3-2a7 7 0 0 1-2-1.2l-1.9.7-2-3.4 1.6-1.2a7 7 0 0 1 0-2.4L4.6 9.9l2-3.4 1.9.7a7 7 0 0 1 2-1.2l.3-2h4l.3 2a7 7 0 0 1 2 1.2l1.9-.7 2 3.4-1.6 1.2a7 7 0 0 1 0 2.4z",
        "workspace" to "M12 19a7 7 0 1 0 0-14 7 7 0 0 0 0 14z",
        "extensions" to "M19.5 11H18V7.5A1.5 1.5 0 0 0 16.5 6H13V4.5a2 2 0 0 0-4 0V6H5.5A1.5 1.5 0 0 0 4 7.5V11h1.5a2 2 0 0 1 0 4H4v3.5A1.5 1.5 0 0 0 5.5 20H9v-1.5a2 2 0 0 1 4 0V20h3.5a1.5 1.5 0 0 0 1.5-1.5V15h1.5a2 2 0 0 0 0-4z",
        "chevron" to "M10 7l5 5-5 5",
        "group" to "M5 5h14v14H5zM12 9v6M9 12h6",
        "update" to "M12 4v11M7.5 10.5L12 15l4.5-4.5M5 19h14",
        "link" to "M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1",
        "copy" to "M9 9h10v10H9zM5 15V5h10",
        "tab" to "M4 6h16v13H4zM4 10h16",
        "image" to "M4 5h16v14H4zM4 16l5-5 4 4 2-2 5 5M15.5 9.5h.01",
    )

    private val parsed = HashMap<String, Path>()

    private val stroke = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        style = Paint.Style.STROKE
        strokeCap = Paint.Cap.ROUND
        strokeJoin = Paint.Join.ROUND
    }
    private val dots = Paint(Paint.ANTI_ALIAS_FLAG)

    /** Draws icon name in color, size px square, centred on cx, cy. "more" is ⋮, as on the web. */
    fun draw(canvas: Canvas, name: String, color: Int, size: Float, cx: Float, cy: Float) {
        val scale = size / 24f
        canvas.save()
        canvas.translate(cx - size / 2, cy - size / 2)
        canvas.scale(scale, scale)
        if (name == "more") {
            dots.color = color
            for (y in listOf(6f, 12f, 18f)) canvas.drawCircle(12f, y, 1.6f, dots)
        } else {
            stroke.color = color
            stroke.strokeWidth = 1.5f
            canvas.drawPath(parsed.getOrPut(name) { parse(paths.getValue(name)) }, stroke)
        }
        canvas.restore()
    }

    /** An SVG path's data as an Android Path: M L H V C S A Z, absolute and relative. */
    fun parse(d: String): Path {
        val path = Path()
        val tokens = Regex("""[MmLlHhVvCcSsAaZz]|-?(?:\d+\.?\d*|\.\d+)(?:e-?\d+)?""").findAll(d).map { it.value }.toList()
        var i = 0
        var cmd = 'M'
        var x = 0f
        var y = 0f
        var startX = 0f
        var startY = 0f
        var lastCx = 0f
        var lastCy = 0f
        fun num() = tokens[i++].toFloat()
        while (i < tokens.size) {
            if (tokens[i][0].isLetter()) cmd = tokens[i++][0]
            val rel = cmd.isLowerCase()
            val ox = if (rel) x else 0f
            val oy = if (rel) y else 0f
            when (cmd.uppercaseChar()) {
                'M' -> {
                    x = ox + num(); y = oy + num()
                    path.moveTo(x, y)
                    startX = x; startY = y
                    // Pairs after a move are lines.
                    cmd = if (rel) 'l' else 'L'
                }
                'L' -> { x = ox + num(); y = oy + num(); path.lineTo(x, y) }
                'H' -> { x = ox + num(); path.lineTo(x, y) }
                'V' -> { y = oy + num(); path.lineTo(x, y) }
                'C' -> {
                    val x1 = ox + num(); val y1 = oy + num()
                    lastCx = ox + num(); lastCy = oy + num()
                    x = ox + num(); y = oy + num()
                    path.cubicTo(x1, y1, lastCx, lastCy, x, y)
                }
                'S' -> {
                    val x1 = 2 * x - lastCx; val y1 = 2 * y - lastCy
                    lastCx = ox + num(); lastCy = oy + num()
                    x = ox + num(); y = oy + num()
                    path.cubicTo(x1, y1, lastCx, lastCy, x, y)
                }
                'A' -> {
                    val rx = num(); val ry = num(); val rot = num()
                    val large = num() != 0f; val sweep = num() != 0f
                    val x2 = ox + num(); val y2 = oy + num()
                    arc(path, x, y, rx, ry, rot, large, sweep, x2, y2)
                    x = x2; y = y2
                }
                'Z' -> { path.close(); x = startX; y = startY }
            }
        }
        return path
    }

    // An SVG arc from x1, y1 to x2, y2, by its centre (SVG 1.1, appendix F.6.5). The icons'
    // arcs are circles, so the rotation is ignored.
    @Suppress("UNUSED_PARAMETER")
    private fun arc(path: Path, x1: Float, y1: Float, rxIn: Float, ryIn: Float, rot: Float,
                    large: Boolean, sweep: Boolean, x2: Float, y2: Float) {
        if (x1 == x2 && y1 == y2) return
        var rx = abs(rxIn).toDouble()
        var ry = abs(ryIn).toDouble()
        val dx = (x1 - x2) / 2.0
        val dy = (y1 - y2) / 2.0
        val lambda = (dx * dx) / (rx * rx) + (dy * dy) / (ry * ry)
        if (lambda > 1) { rx *= sqrt(lambda); ry *= sqrt(lambda) }
        val num = rx * rx * ry * ry - rx * rx * dy * dy - ry * ry * dx * dx
        val den = rx * rx * dy * dy + ry * ry * dx * dx
        var coef = sqrt(maxOf(0.0, num / den))
        if (large == sweep) coef = -coef
        val cxp = coef * rx * dy / ry
        val cyp = -coef * ry * dx / rx
        val cx = cxp + (x1 + x2) / 2.0
        val cy = cyp + (y1 + y2) / 2.0
        fun angle(ux: Double, uy: Double, vx: Double, vy: Double): Double {
            val dot = ux * vx + uy * vy
            val len = sqrt(ux * ux + uy * uy) * sqrt(vx * vx + vy * vy)
            var a = acos((dot / len).coerceIn(-1.0, 1.0))
            if (ux * vy - uy * vx < 0) a = -a
            return a
        }
        val start = angle(1.0, 0.0, (dx - cxp) / rx, (dy - cyp) / ry)
        var delta = angle((dx - cxp) / rx, (dy - cyp) / ry, (-dx - cxp) / rx, (-dy - cyp) / ry)
        if (!sweep && delta > 0) delta -= 2 * PI
        if (sweep && delta < 0) delta += 2 * PI
        val oval = RectF((cx - rx).toFloat(), (cy - ry).toFloat(), (cx + rx).toFloat(), (cy + ry).toFloat())
        val deg = 180 / PI
        // arcTo with a full circle draws nothing: split it.
        if (abs(delta) >= 2 * PI - 1e-6) {
            path.arcTo(oval, (start * deg).toFloat(), (delta / 2 * deg).toFloat())
            path.arcTo(oval, ((start + delta / 2) * deg).toFloat(), (delta / 2 * deg).toFloat())
        } else {
            path.arcTo(oval, (start * deg).toFloat(), (delta * deg).toFloat())
        }
        // The arc ends exactly where SVG says, whatever the float error.
        path.lineTo(x2, y2)
    }
}

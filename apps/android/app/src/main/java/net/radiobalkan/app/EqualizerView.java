package net.radiobalkan.app;

import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Paint;
import android.view.View;

public final class EqualizerView extends View {
    private final Paint paint = new Paint(Paint.ANTI_ALIAS_FLAG);
    private boolean active = true;
    private int barCount = 13;
    private boolean accentOnly;

    public EqualizerView(Context context) {
        super(context);
        paint.setStrokeCap(Paint.Cap.ROUND);
    }

    public void setActive(boolean value) { if (active == value) return; active = value; invalidate(); }
    public void setBarCount(int value) { int next = Math.max(3, Math.min(24, value)); if (barCount == next) return; barCount = next; invalidate(); }
    public void setAccentOnly(boolean value) { if (accentOnly == value) return; accentOnly = value; invalidate(); }

    @Override protected void onDraw(Canvas canvas) {
        super.onDraw(canvas);
        int w = getWidth(), h = getHeight();
        if (w <= 0 || h <= 0) return;
        float gap = Math.max(2f, w * 0.018f);
        float bw = Math.max(3f, (w - gap * (barCount - 1)) / barCount);
        float[] shape = {0.34f,0.48f,0.64f,0.82f,0.56f,0.72f,0.93f,0.61f,0.78f,0.52f,0.88f,0.58f,0.40f,0.68f,0.47f,0.76f};
        for (int i=0;i<barCount;i++) {
            float f = shape[i % shape.length] * (active ? 1f : 0.38f);
            float bh = Math.max(5f, h * f);
            float left = i * (bw + gap);
            int color;
            if (accentOnly) color = 0xFFFFAA32;
            else if (i < barCount * 0.32f) color = 0xFFF05A35;
            else if (i < barCount * 0.68f) color = 0xFFFFAE32;
            else if (i < barCount * 0.84f) color = 0xFF98A36B;
            else color = 0xFF4E5661;
            paint.setColor(color);
            paint.setAlpha(active ? 255 : 135);
            canvas.drawRoundRect(left, h-bh, left+bw, h, bw/2f, bw/2f, paint);
        }
    }
}

package net.radiobalkan.app;

import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Paint;
import android.graphics.Path;
import android.graphics.PixelFormat;
import android.graphics.Rect;
import android.graphics.drawable.Drawable;
import java.util.Locale;

/** Lightweight flag renderer used throughout the UI; no network images or emoji dependency. */
public final class CountryFlagDrawable extends Drawable {
    private final String code;
    private final Paint p = new Paint(Paint.ANTI_ALIAS_FLAG);
    private int alpha = 255;

    public CountryFlagDrawable(String value) {
        code = value == null ? "" : value.trim().toUpperCase(Locale.ROOT);
    }

    @Override public void draw(Canvas c) {
        Rect b = getBounds();
        if (b.width() <= 0 || b.height() <= 0) return;
        c.save();
        c.clipRect(b);
        int l=b.left,t=b.top,r=b.right,bt=b.bottom,w=b.width(),h=b.height();
        fill(c,l,t,r,bt,0xFFF1F1F1);
        switch (code) {
            case "HR":
                h3(c,l,t,r,bt,0xFFFF0000,0xFFFFFFFF,0xFF171796);
                int sx=l+w/2-4, sy=t+h/2-4;
                for(int y=0;y<4;y++) for(int x=0;x<4;x++) fill(c,sx+x*2,sy+y*2,sx+x*2+2,sy+y*2+2,((x+y)&1)==0?0xFFDD0000:0xFFFFFFFF);
                break;
            case "BA":
                fill(c,l,t,r,bt,0xFF002F6C); tri(c,l+w/3,t+1,r-1,t+1,r-1,bt-1,0xFFFFCD00);
                p.setColor(0xFFFFFFFF); for(int i=0;i<4;i++) c.drawCircle(l+4+i*w/6f,t+3+i*h/5f,Math.max(1,h/12f),p);
                break;
            case "RS":
                h3(c,l,t,r,bt,0xFFC6363C,0xFF0C4076,0xFFFFFFFF); fill(c,l+w/5,t+2,l+w/5+Math.max(3,w/7),bt-2,0xFFB62028); break;
            case "SI":
                h3(c,l,t,r,bt,0xFFFFFFFF,0xFF0054A6,0xFFD52B1E); shield(c,l+w/5,t+2,Math.max(5,w/5),Math.max(7,h/2)); break;
            case "MK":
                fill(c,l,t,r,bt,0xFFD2001E); sun(c,l+w/2,t+h/2,Math.max(2,h/5),0xFFFFE300); break;
            case "AL":
                fill(c,l,t,r,bt,0xFFDA121A); eagle(c,l+w/2,t+h/2,Math.max(3,w/5),0xFF111111); break;
            case "ME":
                fill(c,l,t,r,bt,0xFFC41E3A); stroke(c,l,t,r,bt,0xFFDAB437,Math.max(1,h/12)); p.setColor(0xFFDAB437); c.drawCircle(l+w/2f,t+h/2f,Math.max(2,h/6f),p); break;
            default:
                fill(c,l,t,r,bt,0xFF263746); p.setColor(0xFFF7A63D); c.drawCircle(l+w/2f,t+h/2f,Math.max(2,h/4f),p); break;
        }
        stroke(c,l,t,r,bt,0x806F625B,1);
        c.restore();
    }

    private void fill(Canvas c,int l,int t,int r,int b,int color){ p.setStyle(Paint.Style.FILL); p.setColor(withAlpha(color)); c.drawRect(l,t,r,b,p); }
    private void stroke(Canvas c,int l,int t,int r,int b,int color,int width){ p.setStyle(Paint.Style.STROKE); p.setStrokeWidth(width); p.setColor(withAlpha(color)); c.drawRect(l+.5f,t+.5f,r-.5f,b-.5f,p); p.setStyle(Paint.Style.FILL); }
    private int withAlpha(int color){ return (color & 0x00FFFFFF) | (alpha << 24); }
    private void h3(Canvas c,int l,int t,int r,int b,int a,int m,int z){ int h=b-t; fill(c,l,t,r,t+h/3,a); fill(c,l,t+h/3,r,t+2*h/3,m); fill(c,l,t+2*h/3,r,b,z); }
    private void tri(Canvas c,float x1,float y1,float x2,float y2,float x3,float y3,int color){ Path path=new Path(); path.moveTo(x1,y1); path.lineTo(x2,y2); path.lineTo(x3,y3); path.close(); p.setColor(withAlpha(color)); c.drawPath(path,p); }
    private void shield(Canvas c,int x,int y,int w,int h){ Path q=new Path(); q.moveTo(x,y); q.lineTo(x+w,y); q.lineTo(x+w-1,y+h*0.65f); q.lineTo(x+w/2f,y+h); q.lineTo(x+1,y+h*0.65f); q.close(); p.setColor(0xFF0054A6); c.drawPath(q,p); p.setColor(Color.WHITE); c.drawLine(x+2,y+h*.55f,x+w/2f,y+h*.25f,p); c.drawLine(x+w/2f,y+h*.25f,x+w-2,y+h*.55f,p); }
    private void sun(Canvas c,float cx,float cy,float rad,int color){ p.setColor(withAlpha(color)); p.setStrokeWidth(Math.max(1,rad/2)); for(int i=0;i<8;i++){ double a=i*Math.PI/4; c.drawLine(cx,cy,(float)(cx+Math.cos(a)*rad*3),(float)(cy+Math.sin(a)*rad*3),p);} c.drawCircle(cx,cy,rad,p); }
    private void eagle(Canvas c,float cx,float cy,float size,int color){ p.setColor(withAlpha(color)); Path q=new Path(); q.moveTo(cx,cy-size*.65f); q.lineTo(cx+size*.35f,cy-size*.15f); q.lineTo(cx+size*.8f,cy-size*.35f); q.lineTo(cx+size*.55f,cy+.1f*size); q.lineTo(cx+size*.8f,cy+size*.5f); q.lineTo(cx+size*.2f,cy+size*.3f); q.lineTo(cx,cy+size*.65f); q.lineTo(cx-size*.2f,cy+size*.3f); q.lineTo(cx-size*.8f,cy+size*.5f); q.lineTo(cx-size*.55f,cy+.1f*size); q.lineTo(cx-size*.8f,cy-size*.35f); q.lineTo(cx-size*.35f,cy-size*.15f); q.close(); c.drawPath(q,p); }

    @Override public void setAlpha(int value){ alpha=Math.max(0,Math.min(255,value)); invalidateSelf(); }
    @Override public void setColorFilter(android.graphics.ColorFilter colorFilter){ p.setColorFilter(colorFilter); invalidateSelf(); }
    @Override public int getOpacity(){ return PixelFormat.TRANSLUCENT; }
    @Override public int getIntrinsicWidth(){ return 28; }
    @Override public int getIntrinsicHeight(){ return 18; }
}

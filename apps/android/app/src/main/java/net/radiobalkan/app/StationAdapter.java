package net.radiobalkan.app;

import android.content.Context;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.BaseAdapter;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.TextView;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;

public final class StationAdapter extends BaseAdapter {
    public interface Actions {
        void onPlay(RadioStation s);
        void onFavorite(RadioStation s);
        void onMore(RadioStation s);
    }

    private final Context context;
    private final Actions actions;
    private final ImageLoader images;
    private List<RadioStation> items = new ArrayList<>();
    private String currentKey = "";
    private boolean playing;

    public StationAdapter(Context context, Actions actions, ImageLoader images) {
        this.context = context;
        this.actions = actions;
        this.images = images;
    }

    public void setPlayback(String key, boolean isPlaying) {
        String nextKey = key == null ? "" : key;
        if (currentKey.equals(nextKey) && playing == isPlaying) return;
        currentKey = nextKey;
        playing = isPlaying;
        notifyDataSetChanged();
    }

    public void setSnapshot(List<RadioStation> value, String key, boolean isPlaying) {
        items = value == null ? new ArrayList<>() : new ArrayList<>(value);
        currentKey = key == null ? "" : key;
        playing = isPlaying;
        notifyDataSetChanged();
    }

    @Override public int getCount() { return items.size(); }
    @Override public RadioStation getItem(int position) { return items.get(position); }
    @Override public long getItemId(int position) {
        String key = getItem(position).key();
        long hash = 0xcbf29ce484222325L;
        for (int i = 0; i < key.length(); i++) { hash ^= key.charAt(i); hash *= 0x100000001b3L; }
        return hash;
    }
    @Override public boolean hasStableIds() { return true; }

    @Override public View getView(int position, View convertView, ViewGroup parent) {
        Row row;
        if (convertView instanceof LinearLayout && convertView.getTag() instanceof Row) {
            row = (Row) convertView.getTag();
        } else {
            row = createRow();
            convertView = row.root;
            convertView.setTag(row);
        }
        RadioStation s = getItem(position);
        String key = s.key();
        boolean active = key.equals(currentKey);

        row.name.setText(s.name);
        row.meta.setText(countryAndGenre(s));
        CountryFlagDrawable flag = new CountryFlagDrawable(s.countryCode);
        flag.setBounds(0, 0, dp(22), dp(14));
        row.meta.setCompoundDrawablePadding(dp(6));
        row.meta.setCompoundDrawables(flag, null, null, null);
        row.listeners.setText(formatListeners(s.votes));
        row.play.setText(active && playing ? "Ⅱ" : "▶");
        row.root.setBackground(rounded(active ? 0xFF171C24 : 0xFF11171F, active ? 0xFFFFA52E : 0xFF303845, 18));

        row.logo.setImageResource(R.drawable.ic_radio_balkan);
        images.load(s.favicon, row.logo, null);
        row.play.setContentDescription((active && playing ? "Pauziraj " : "Slušaj ") + s.name);
        row.play.setOnClickListener(v -> actions.onPlay(s));
        row.root.setOnClickListener(v -> actions.onPlay(s));
        row.root.setOnLongClickListener(v -> { actions.onMore(s); return true; });
        return convertView;
    }

    private Row createRow() {
        Row r = new Row();
        LinearLayout root = new LinearLayout(context);
        root.setOrientation(LinearLayout.HORIZONTAL);
        root.setGravity(Gravity.CENTER_VERTICAL);
        root.setPadding(0, 0, dp(12), 0);
        LinearLayout.LayoutParams rp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(112));
        rp.setMargins(dp(1), dp(5), dp(1), dp(5));
        root.setLayoutParams(rp);
        root.setBackground(rounded(0xFF11171F, 0xFF303845, 18));
        root.setClipToOutline(true);

        FrameLayout artBox = new FrameLayout(context);
        artBox.setBackground(rounded(0xFF1C2430, 0xFF303845, 18));
        artBox.setClipToOutline(true);
        ImageView logo = new ImageView(context);
        logo.setScaleType(ImageView.ScaleType.CENTER_CROP);
        artBox.addView(logo, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        root.addView(artBox, new LinearLayout.LayoutParams(dp(122), ViewGroup.LayoutParams.MATCH_PARENT));
        r.logo = logo;

        LinearLayout info = new LinearLayout(context);
        info.setOrientation(LinearLayout.VERTICAL);
        info.setGravity(Gravity.CENTER_VERTICAL);
        info.setPadding(dp(13), 0, dp(5), 0);
        r.name = text(17, Color.WHITE, true);
        r.meta = text(12, 0xFFBEC4CD, false);
        info.addView(r.name, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(34)));
        info.addView(r.meta, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(28)));
        root.addView(info, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));

        LinearLayout side = new LinearLayout(context);
        side.setOrientation(LinearLayout.VERTICAL);
        side.setGravity(Gravity.CENTER);
        r.listeners = text(11, 0xFFB7BEC8, false);
        r.listeners.setGravity(Gravity.CENTER);
        android.graphics.drawable.Drawable people = context.getDrawable(R.drawable.ic_people);
        if (people != null) {
            people.setBounds(0, 0, dp(16), dp(16));
            r.listeners.setCompoundDrawablePadding(dp(4));
            r.listeners.setCompoundDrawables(people, null, null, null);
        }
        side.addView(r.listeners, new LinearLayout.LayoutParams(dp(62), dp(34)));
        root.addView(side, new LinearLayout.LayoutParams(dp(66), ViewGroup.LayoutParams.MATCH_PARENT));

        r.play = smallButton("▶", true, 18);
        root.addView(r.play, new LinearLayout.LayoutParams(dp(60), dp(60)));
        r.root = root;
        return r;
    }

    private String countryAndGenre(RadioStation s) {
        String country = s.country == null || s.country.trim().isEmpty() ? RadioRepository.countryName(s.countryCode) : s.country.trim();
        String genre = firstGenre(s.tags);
        if (country.isEmpty()) return genre.isEmpty() ? "Radio uživo" : genre;
        return genre.isEmpty() ? country : country + "  •  " + genre;
    }

    private static String firstGenre(String tags) {
        if (tags == null) return "";
        for (String raw : tags.split(",")) {
            String v = raw.trim();
            if (v.length() >= 2 && v.length() <= 18) {
                String lower = v.toLowerCase(Locale.ROOT);
                if (!lower.contains("radio") && !lower.contains("music")) return Character.toUpperCase(v.charAt(0)) + v.substring(1);
            }
        }
        return "";
    }


    private static String formatListeners(int votes) {
        int n = Math.max(0, votes);
        if (n >= 1000) {
            double k = n / 1000.0;
            return k >= 10 ? String.format(Locale.ROOT, "%.0fK", k) : String.format(Locale.ROOT, "%.1fK", k);
        }
        return String.valueOf(n);
    }

    private Button smallButton(String label, boolean accent, int textSize) {
        Button b = new Button(context);
        b.setAllCaps(false);
        b.setText(label);
        b.setTextSize(textSize);
        b.setTextColor(accent ? 0xFFF8F8FA : 0xFFCAD0D8);
        b.setPadding(0, 0, 0, 0);
        b.setMinWidth(0); b.setMinimumWidth(0); b.setMinHeight(0); b.setMinimumHeight(0);
        b.setBackground(rounded(accent ? 0xFF25242A : 0x00000000, accent ? 0xFF81592C : 0x00000000, accent ? 30 : 10));
        return b;
    }

    private TextView text(int size, int color, boolean bold) {
        TextView v = new TextView(context);
        v.setTextSize(size); v.setTextColor(color); v.setGravity(Gravity.CENTER_VERTICAL);
        if (bold) v.setTypeface(Typeface.DEFAULT_BOLD);
        v.setSingleLine(true); v.setEllipsize(android.text.TextUtils.TruncateAt.END);
        return v;
    }

    private GradientDrawable rounded(int fill, int stroke, int radiusDp) {
        GradientDrawable g = new GradientDrawable();
        g.setColor(fill); g.setCornerRadius(dp(radiusDp));
        if ((stroke >>> 24) != 0) g.setStroke(dp(1), stroke);
        return g;
    }

    private int dp(int v) { return (int) (v * context.getResources().getDisplayMetrics().density + 0.5f); }

    private static final class Row {
        LinearLayout root;
        ImageView logo;
        TextView name, meta, listeners;
        Button play;
    }
}

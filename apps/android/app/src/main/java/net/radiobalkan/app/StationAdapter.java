package net.radiobalkan.app;

import android.content.Context;
import android.content.res.ColorStateList;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.graphics.drawable.RippleDrawable;
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
        void onDetails(RadioStation s);
        void onFavorite(RadioStation s);
        void onMore(RadioStation s);
    }

    private final Context context;
    private final Actions actions;
    private final ImageLoader images;
    private List<RadioStation> items = new ArrayList<>();
    private String currentKey = "";
    private boolean playing;
    private boolean adminMode;

    public StationAdapter(Context context, Actions actions, ImageLoader images) {
        this.context = context;
        this.actions = actions;
        this.images = images;
    }

    public void setAdminMode(boolean enabled) {
        if (adminMode == enabled) return;
        adminMode = enabled;
        notifyDataSetChanged();
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
        row.listeners.setText(statusLine(s));
        row.play.setText(active && playing ? "Ⅱ" : "▶");
        row.root.setSelected(active);
        row.root.setBackground(interactiveRounded(active ? 0xFF171C24 : 0xFF11171F, active ? 0xFFFFA52E : 0xFF303845, 18, 0x26FFFFFF));

        row.logo.setImageResource(R.drawable.ic_radio_balkan);
        images.load(s.favicon, row.logo, null);
        String action = active && playing ? "Pauziraj " : "Slušaj ";
        row.play.setContentDescription(action + s.name);
        row.more.setContentDescription("Više opcija za " + s.name);
        row.root.setContentDescription(s.name + ", " + countryAndGenre(s) + ", " + statusLine(s) + (active ? ", trenutno odabrana" : ""));
        row.play.setOnClickListener(v -> actions.onPlay(s));
        row.more.setOnClickListener(v -> actions.onMore(s));
        row.root.setOnClickListener(v -> actions.onDetails(s));
        row.root.setOnLongClickListener(v -> { actions.onMore(s); return true; });
        return convertView;
    }

    private Row createRow() {
        Row r = new Row();
        LinearLayout root = new LinearLayout(context);
        root.setOrientation(LinearLayout.HORIZONTAL);
        root.setGravity(Gravity.CENTER_VERTICAL);
        root.setPadding(0, 0, dp(8), 0);
        root.setFocusable(true);
        root.setClickable(true);
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
        root.addView(artBox, new LinearLayout.LayoutParams(dp(104), ViewGroup.LayoutParams.MATCH_PARENT));
        r.logo = logo;

        LinearLayout info = new LinearLayout(context);
        info.setOrientation(LinearLayout.VERTICAL);
        info.setGravity(Gravity.CENTER_VERTICAL);
        info.setPadding(dp(13), 0, dp(6), 0);
        r.name = text(17, Color.WHITE, true);
        r.meta = text(12, 0xFFBEC4CD, false);
        r.listeners = text(11, 0xFF98A2AF, false);
        info.addView(r.name, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(31)));
        info.addView(r.meta, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(26)));
        info.addView(r.listeners, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(23)));
        root.addView(info, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));

        r.more = smallButton("⋮", false, 24);
        root.addView(r.more, new LinearLayout.LayoutParams(dp(46), dp(48)));

        r.play = smallButton("▶", true, 18);
        root.addView(r.play, new LinearLayout.LayoutParams(dp(56), dp(56)));
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

    private String statusLine(RadioStation s) {
        String popularity = formatPopularity(s.votes);
        if (!adminMode) return popularity;
        String health;
        if ("ok".equals(s.health)) health = s.replaced ? "Dostupno · zamjenski izvor" : "Dostupno";
        else if ("checking".equals(s.health)) health = "Provjeravam dostupnost";
        else if ("broken".equals(s.health)) health = "Trenutno nedostupno";
        else health = "Dostupnost nije provjerena";
        return health + "  •  " + popularity;
    }

    private static String formatPopularity(int votes) {
        int n = Math.max(0, votes);
        String value;
        if (n >= 1000) {
            double k = n / 1000.0;
            value = k >= 10 ? String.format(Locale.ROOT, "%.0fK", k) : String.format(Locale.ROOT, "%.1fK", k);
        } else {
            value = String.valueOf(n);
        }
        return "Popularnost · " + value + " glasova";
    }

    private Button smallButton(String label, boolean accent, int textSize) {
        Button b = new Button(context);
        b.setAllCaps(false);
        b.setText(label);
        b.setTextSize(textSize);
        b.setTextColor(accent ? 0xFFF8F8FA : 0xFFCAD0D8);
        b.setPadding(0, 0, 0, 0);
        b.setMinWidth(0); b.setMinimumWidth(0); b.setMinHeight(0); b.setMinimumHeight(0);
        b.setBackground(interactiveRounded(accent ? 0xFF25242A : 0x00000000, accent ? 0xFF81592C : 0x00000000, accent ? 28 : 12, 0x30FFFFFF));
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

    private android.graphics.drawable.Drawable interactiveRounded(int fill, int stroke, int radiusDp, int rippleColor) {
        GradientDrawable content = rounded(fill, stroke, radiusDp);
        GradientDrawable mask = rounded(Color.WHITE, 0x00000000, radiusDp);
        return new RippleDrawable(ColorStateList.valueOf(rippleColor), content, mask);
    }

    private int dp(int v) { return (int) (v * context.getResources().getDisplayMetrics().density + 0.5f); }

    private static final class Row {
        LinearLayout root;
        ImageView logo;
        TextView name, meta, listeners;
        Button play, more;
    }
}

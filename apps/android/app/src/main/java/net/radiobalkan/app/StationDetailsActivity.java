package net.radiobalkan.app;

import android.app.Activity;
import android.content.Intent;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.net.Uri;
import android.os.Bundle;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.Button;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;
import android.widget.Toast;
import org.json.JSONObject;

/** Dedicated public station page. This Activity is intentionally not exported. */
public final class StationDetailsActivity extends Activity {
    public static final String EXTRA_STATION_JSON = "net.radiobalkan.app.STATION_JSON";
    public static final String EXTRA_PLAY_STATION_JSON = "net.radiobalkan.app.PLAY_STATION_JSON";

    private StateStore state;
    private ImageLoader images;
    private RadioStation station;
    private Button favoriteButton;

    @Override protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        state = new StateStore(this);
        images = new ImageLoader();
        station = readStation(getIntent());
        if (station == null || station.name == null || station.name.trim().isEmpty()) {
            finish();
            return;
        }
        buildUi();
    }

    private RadioStation readStation(Intent intent) {
        try {
            String json = intent == null ? "" : intent.getStringExtra(EXTRA_STATION_JSON);
            if (json == null || json.trim().isEmpty() || json.length() > 32_768) return null;
            return RadioStation.fromJson(new JSONObject(json));
        } catch (Throwable error) {
            AppLog.e(this, "station-details-input", error);
            return null;
        }
    }

    private void buildUi() {
        ScrollView scroll = new ScrollView(this);
        scroll.setFillViewport(true);
        scroll.setBackgroundColor(0xFF090E14);

        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setPadding(dp(18), dp(16), dp(18), dp(28));
        scroll.addView(root, new ScrollView.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));
        setContentView(scroll);

        Button back = button("← Sve stanice", false);
        back.setContentDescription("Natrag na popis radio stanica");
        back.setOnClickListener(v -> finish());
        root.addView(back, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, dp(44)));

        LinearLayout hero = new LinearLayout(this);
        hero.setGravity(Gravity.CENTER_VERTICAL);
        hero.setPadding(dp(16), dp(16), dp(16), dp(16));
        hero.setBackground(panel(0xFF161B23, 0xFF6B4A2C, 20));

        ImageView logo = new ImageView(this);
        logo.setScaleType(ImageView.ScaleType.CENTER_CROP);
        logo.setImageResource(R.drawable.ic_radio_balkan);
        hero.addView(logo, new LinearLayout.LayoutParams(dp(92), dp(92)));
        images.load(station.favicon, logo, null);

        LinearLayout heading = new LinearLayout(this);
        heading.setOrientation(LinearLayout.VERTICAL);
        heading.setPadding(dp(15), 0, 0, 0);
        TextView live = text("●  UŽIVO", 11, 0xFFFFB23F, true);
        heading.addView(live);
        TextView title = text(station.name, 25, Color.WHITE, true);
        title.setSingleLine(false);
        heading.addView(title, margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 6, 0, 4));
        TextView meta = text(StationPresentation.area(station) + " · " + stationMeta(), 13, 0xFFB7C0CB, false);
        meta.setSingleLine(false);
        heading.addView(meta);
        hero.addView(heading, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));

        root.addView(hero, margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 14, 0, 0));

        LinearLayout actions = new LinearLayout(this);
        actions.setGravity(Gravity.CENTER_VERTICAL);
        Button play = button("▶  Slušaj uživo", true);
        play.setContentDescription("Slušaj " + station.name);
        play.setOnClickListener(v -> requestPlayback());
        actions.addView(play, new LinearLayout.LayoutParams(0, dp(52), 1.3f));

        favoriteButton = button("", false);
        updateFavoriteButton();
        favoriteButton.setOnClickListener(v -> {
            boolean on = state.toggleFavorite(station.key());
            updateFavoriteButton();
            Toast.makeText(this, on ? "Dodano u omiljene" : "Uklonjeno iz omiljenih", Toast.LENGTH_SHORT).show();
        });
        LinearLayout.LayoutParams favLp = new LinearLayout.LayoutParams(0, dp(52), 1f);
        favLp.setMargins(dp(9), 0, 0, 0);
        actions.addView(favoriteButton, favLp);
        root.addView(actions, margin(ViewGroup.LayoutParams.MATCH_PARENT, dp(52), 0, 10, 0, 0));

        LinearLayout about = new LinearLayout(this);
        about.setOrientation(LinearLayout.VERTICAL);
        about.setPadding(dp(16), dp(16), dp(16), dp(17));
        about.setBackground(panel(0xFF101720, 0xFF2E3946, 17));
        about.addView(text("O radio stanici", 18, Color.WHITE, true));
        TextView description = text(StationPresentation.description(station), 14, 0xFFE0E5EB, false);
        description.setSingleLine(false);
        description.setLineSpacing(0f, 1.14f);
        about.addView(description, margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 10, 0, 0));

        TextView facts = text(StationPresentation.publicDetails(station), 12, 0xFFA2ADB9, false);
        facts.setSingleLine(false);
        facts.setPadding(dp(12), dp(10), dp(12), dp(10));
        facts.setBackground(panel(0xFF0D141C, 0xFF2C3845, 12));
        about.addView(facts, margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 13, 0, 0));
        root.addView(about, margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 12, 0, 0));

        TextView privacy = text("Stream adresa i administratorski maintenance podaci nisu prikazani na javnoj stranici stanice.", 11, 0xFF788493, false);
        privacy.setGravity(Gravity.CENTER);
        privacy.setSingleLine(false);
        root.addView(privacy, margin(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT, 0, 16, 0, 0));

        TextView brendigo = text("Built with Brendigo", 11, 0xFFD6A461, true);
        brendigo.setGravity(Gravity.CENTER);
        brendigo.setPadding(dp(12), dp(16), dp(12), dp(16));
        brendigo.setClickable(true);
        brendigo.setFocusable(true);
        brendigo.setContentDescription("Built with Brendigo, otvori brendigo.com");
        brendigo.setOnClickListener(v -> openBrendigo());
        root.addView(brendigo, margin(ViewGroup.LayoutParams.MATCH_PARENT, dp(52), 0, 12, 0, 0));
    }

    private String stationMeta() {
        String value = StationPresentation.firstUsefulTag(station.tags);
        if (value.isEmpty()) value = station.codec == null ? "" : station.codec.trim().toUpperCase();
        if (station.bitrate > 0) value = value.isEmpty() ? station.bitrate + " kbps" : value + " · " + station.bitrate + " kbps";
        return value.isEmpty() ? "Radio uživo" : value;
    }

    private void requestPlayback() {
        Intent intent = new Intent(this, MainActivity.class)
                .addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP | Intent.FLAG_ACTIVITY_SINGLE_TOP)
                .putExtra(EXTRA_PLAY_STATION_JSON, station.toJson().toString());
        try {
            startActivity(intent);
            finish();
        } catch (Throwable error) {
            AppLog.e(this, "station-details-play", error);
            Toast.makeText(this, "Reprodukciju trenutačno nije moguće pokrenuti", Toast.LENGTH_SHORT).show();
        }
    }

    private void updateFavoriteButton() {
        if (favoriteButton == null) return;
        boolean favorite = state.isFavorite(station.key());
        favoriteButton.setText(favorite ? "♥ Omiljena" : "♡ Dodaj u omiljene");
        favoriteButton.setContentDescription(favorite ? "Ukloni iz omiljenih" : "Dodaj u omiljene");
    }

    private void openBrendigo() {
        try {
            startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse("https://brendigo.com/")));
        } catch (Throwable error) {
            AppLog.e(this, "station-details-brendigo", error);
            Toast.makeText(this, "Brendigo web stranica trenutačno nije dostupna", Toast.LENGTH_SHORT).show();
        }
    }

    @Override protected void onDestroy() {
        if (images != null) images.close();
        super.onDestroy();
    }

    private TextView text(String value, float size, int color, boolean bold) {
        TextView view = new TextView(this);
        view.setText(value);
        view.setTextSize(size);
        view.setTextColor(color);
        if (bold) view.setTypeface(Typeface.DEFAULT_BOLD);
        return view;
    }

    private Button button(String value, boolean primary) {
        Button button = new Button(this);
        button.setText(value);
        button.setAllCaps(false);
        button.setTextSize(13);
        button.setTypeface(Typeface.DEFAULT_BOLD);
        button.setTextColor(primary ? 0xFF16110B : 0xFFE1E5EA);
        button.setBackground(panel(primary ? 0xFFFFB23F : 0xFF111820, primary ? 0xFFFFCA72 : 0xFF374353, 13));
        button.setMinHeight(0);
        button.setMinimumHeight(0);
        return button;
    }

    private GradientDrawable panel(int fill, int stroke, int radiusDp) {
        GradientDrawable d = new GradientDrawable();
        d.setColor(fill);
        d.setCornerRadius(dp(radiusDp));
        d.setStroke(dp(1), stroke);
        return d;
    }

    private LinearLayout.LayoutParams margin(int width, int height, int left, int top, int right, int bottom) {
        LinearLayout.LayoutParams p = new LinearLayout.LayoutParams(width, height);
        p.setMargins(dp(left), dp(top), dp(right), dp(bottom));
        return p;
    }

    private int dp(int value) {
        return Math.round(value * getResources().getDisplayMetrics().density);
    }
}

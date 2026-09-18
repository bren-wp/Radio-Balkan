package net.radiobalkan.app;

import android.Manifest;
import android.app.Activity;
import android.app.AlertDialog;
import android.content.BroadcastReceiver;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.pm.PackageManager;
import android.content.res.ColorStateList;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.graphics.drawable.RippleDrawable;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.text.Editable;
import android.text.TextWatcher;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.view.WindowInsets;
import android.view.inputmethod.InputMethodManager;
import android.widget.Button;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.HorizontalScrollView;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ListView;
import android.widget.TextView;
import android.widget.Toast;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.RejectedExecutionException;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;

public final class MainActivity extends Activity implements StationAdapter.Actions {
    private static final int PLAYER_H_DP = 112;
    private static final int NAV_H_DP = 70;
    private final Handler ui = new Handler(Looper.getMainLooper());
    private final ExecutorService filterWorker = Executors.newSingleThreadExecutor(r -> new Thread(r, "radio-filter"));
    private final ExecutorService ioWorker = Executors.newFixedThreadPool(2, r -> {
        Thread t = new Thread(r, "radio-io");
        t.setPriority(Thread.MIN_PRIORITY);
        return t;
    });
    private final AtomicInteger filterGeneration = new AtomicInteger();
    private final AtomicInteger catalogGeneration = new AtomicInteger();
    private final AtomicInteger healthGeneration = new AtomicInteger();
    private final AtomicBoolean healthRunning = new AtomicBoolean();
    private final AtomicBoolean autoHealthStarted = new AtomicBoolean();
    private final AtomicBoolean catalogRefreshRunning = new AtomicBoolean();
    private final Object dataLock = new Object();
    private List<RadioStation> allStations = new ArrayList<>();
    private List<RadioStation> visibleStations = new ArrayList<>();
    private RadioRepository repository;
    private StateStore state;
    private StationAdapter adapter;
    private ImageLoader images;
    private EditText search;
    private TextView heroName, heroMeta, statusText, playerName, playerMeta;
    private ImageView playerArtwork;
    private View searchBox;
    private EqualizerView playerEqualizer;
    private Button heroPlay, playerPrev, playerPlay, playerStop, playerNext;
    private ListView list;
    private String country = "";
    private String genre = "";
    private String tab = "all";
    private String query = "";
    private String currentKey = "";
    private String currentCountryCode = "";
    private boolean playing;
    private boolean playbackStopped = true;
    private RadioStation featured;
    private Runnable searchRunnable;
    private boolean receiverRegistered;
    private android.window.OnBackInvokedCallback backInvokedCallback;
    private volatile boolean destroyed;
    private final Map<String, LinearLayout> bottomNavItems = new HashMap<>();
    private String navSelection = "radio";

    private final BroadcastReceiver playerReceiver = new BroadcastReceiver() {
        @Override public void onReceive(Context context, Intent intent) {
            if (!RadioPlayerService.ACTION_STATE.equals(intent.getAction()) || destroyed) return;
            currentKey = safe(intent.getStringExtra(RadioPlayerService.EXTRA_KEY));
            playing = intent.getBooleanExtra(RadioPlayerService.EXTRA_PLAYING, false);
            playbackStopped = intent.getBooleanExtra(RadioPlayerService.EXTRA_STOPPED, false);
            String name = safe(intent.getStringExtra(RadioPlayerService.EXTRA_NAME));
            String meta = safe(intent.getStringExtra(RadioPlayerService.EXTRA_META));
            currentCountryCode = safe(intent.getStringExtra(RadioPlayerService.EXTRA_COUNTRY));
            if (currentCountryCode.isEmpty()) {
                RadioStation match = stationByKey(currentKey);
                if (match != null) currentCountryCode = match.countryCode;
            }
            String now = safe(intent.getStringExtra(RadioPlayerService.EXTRA_NOW_PLAYING));
            String status = safe(intent.getStringExtra(RadioPlayerService.EXTRA_STATUS));
            playerName.setText(name.isEmpty() ? "Odaberi radio stanicu" : name);
            playerMeta.setText(!now.isEmpty() ? now : (meta.isEmpty() ? "Radio uživo" : meta));
            applyFlag(playerMeta, currentCountryCode);
            RadioStation artworkStation = stationByKey(currentKey);
            if (playerArtwork != null && artworkStation != null) {
                playerArtwork.setImageResource(R.drawable.ic_radio_balkan);
                images.load(artworkStation.favicon, playerArtwork, null);
            }
            if (playerEqualizer != null) playerEqualizer.setActive(playing);
            statusText.setText(status.isEmpty() ? "Spremno" : status);
            updatePlaybackControls();
            if (adapter != null) adapter.setPlayback(currentKey, playing);
        }
    };

    @Override protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        state = new StateStore(this);
        country = state.country();
        if (!country.isEmpty() && !RadioRepository.isSupportedCountry(country)) {
            country = "";
            state.setCountry("");
        }
        genre = state.genre();
        tab = validTab(state.tab()) ? state.tab() : "all";
        navSelection = navSelectionForTab(tab);
        images = new ImageLoader();
        repository = new RadioRepository(this);
        buildUi();
        installBackHandler();
        registerPlayerReceiver();
        queryPlayerState();
        loadStations();
    }

    private void buildUi() {
        FrameLayout root = new FrameLayout(this);
        root.setBackgroundColor(0xFF090E14);
        setContentView(root);
        installSystemBarInsets(root);

        LinearLayout content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setPadding(dp(16), dp(8), dp(16), dp(PLAYER_H_DP + NAV_H_DP + 10));
        root.addView(content, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        content.addView(buildTopBar(), new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(70)));
        searchBox = buildSearchBar();
        searchBox.setVisibility(View.GONE);
        content.addView(searchBox, marginParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(50), 0, 0, 0, 8));

        LinearLayout header = new LinearLayout(this);
        header.setOrientation(LinearLayout.VERTICAL);
        header.addView(buildHero(), marginParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(238), 0, 3, 0, 14));

        LinearLayout stationsHeader = new LinearLayout(this);
        stationsHeader.setGravity(Gravity.CENTER_VERTICAL);
        stationsHeader.addView(label("Sve stanice", 25, Color.WHITE, true), new LinearLayout.LayoutParams(0, dp(54), 1f));
        Button sort = chip("Filtriraj ⌄", false);
        sort.setTextSize(14);
        sort.setBackground(interactiveRounded(0x00000000, 0x00000000, 12, 0x24FFFFFF));
        sort.setTextColor(0xFFB9BDC6);
        sort.setOnClickListener(v -> showBrowseDialog());
        stationsHeader.addView(sort, new LinearLayout.LayoutParams(dp(126), dp(46)));
        header.addView(stationsHeader);

        adapter = new StationAdapter(this, this, images);
        list = new ListView(this);
        list.addHeaderView(header, null, false);
        list.setAdapter(adapter);
        list.setDivider(null);
        list.setDividerHeight(0);
        list.setCacheColorHint(Color.TRANSPARENT);
        list.setBackgroundColor(Color.TRANSPARENT);
        list.setClipToPadding(false);
        list.setVerticalScrollBarEnabled(true);
        list.setScrollbarFadingEnabled(true);
        list.setOverScrollMode(View.OVER_SCROLL_NEVER);
        content.addView(list, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f));

        FrameLayout.LayoutParams playerLp = new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(PLAYER_H_DP));
        playerLp.gravity = Gravity.BOTTOM;
        playerLp.bottomMargin = dp(NAV_H_DP);
        root.addView(buildPlayerBar(), playerLp);
        root.addView(buildBottomNav(), bottomNavLayoutParams());
    }

    private View buildTopBar() {
        LinearLayout bar = new LinearLayout(this);
        bar.setGravity(Gravity.CENTER_VERTICAL);
        bar.setPadding(dp(4), 0, dp(2), 0);

        Button back = smallTop("‹");
        back.setTextSize(32);
        back.setTextColor(0xFFE8EBF1);
        back.setBackground(interactiveRounded(0x00000000, 0x00000000, 12, 0x24FFFFFF));
        back.setContentDescription("Natrag");
        back.setOnClickListener(v -> handleBackNavigation());
        bar.addView(back, new LinearLayout.LayoutParams(dp(46), dp(52)));

        EqualizerView mark = new EqualizerView(this);
        mark.setBarCount(5);
        mark.setAccentOnly(true);
        mark.setActive(true);
        LinearLayout.LayoutParams markLp = new LinearLayout.LayoutParams(dp(54), dp(42));
        markLp.setMargins(dp(4), 0, dp(11), 0);
        bar.addView(mark, markLp);

        LinearLayout names = new LinearLayout(this);
        names.setOrientation(LinearLayout.VERTICAL);
        names.addView(label("Radio Balkan", 23, Color.WHITE, true), new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, dp(31)));
        names.addView(label("Glazba koja povezuje regiju.", 12, 0xFF9CA3AF, false), new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, dp(19)));
        bar.addView(names, new LinearLayout.LayoutParams(0, dp(54), 1f));

        Button searchButton = smallTop("⌕");
        searchButton.setTextSize(24);
        searchButton.setBackground(interactiveRounded(0x00000000, 0x00000000, 12, 0x24FFFFFF));
        searchButton.setContentDescription("Pretraži");
        searchButton.setOnClickListener(v -> setSearchVisible(searchBox == null || searchBox.getVisibility() != View.VISIBLE));
        bar.addView(searchButton, new LinearLayout.LayoutParams(dp(48), dp(48)));

        Button menu = smallTop("⋮");
        menu.setTextSize(24);
        menu.setBackground(interactiveRounded(0x00000000, 0x00000000, 12, 0x24FFFFFF));
        menu.setContentDescription("Više opcija");
        menu.setOnClickListener(v -> showAppMenu());
        bar.addView(menu, new LinearLayout.LayoutParams(dp(42), dp(48)));
        return bar;
    }

    private View buildSearchBar() {
        LinearLayout box = new LinearLayout(this);
        box.setGravity(Gravity.CENTER_VERTICAL);
        box.setPadding(dp(14), 0, dp(10), 0);
        box.setBackground(rounded(0xFF111923, 0xFF293342, 18));
        TextView icon = label("⌕", 19, 0xFF9AA4B2, false);
        box.addView(icon, new LinearLayout.LayoutParams(dp(34), ViewGroup.LayoutParams.MATCH_PARENT));
        search = new EditText(this);
        search.setSingleLine(true);
        search.setHint("Pretraži stanice, žanrove, gradove, zemlje…");
        search.setHintTextColor(0xFF687382);
        search.setTextColor(Color.WHITE);
        search.setTextSize(13);
        search.setBackgroundColor(Color.TRANSPARENT);
        box.addView(search, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));
        search.addTextChangedListener(new TextWatcher() {
            @Override public void beforeTextChanged(CharSequence s, int start, int count, int after) { }
            @Override public void onTextChanged(CharSequence s, int start, int before, int count) {
                if (searchRunnable != null) ui.removeCallbacks(searchRunnable);
                final String typed = s == null ? "" : s.toString();
                searchRunnable = () -> { query = RadioStation.fold(typed); applyFilterAsync(); };
                ui.postDelayed(searchRunnable, 140);
            }
            @Override public void afterTextChanged(Editable s) { }
        });
        return box;
    }

    private View buildHero() {
        FrameLayout hero = new FrameLayout(this);
        hero.setBackground(rounded(0xFF10161E, 0xFF4C382A, 22));
        hero.setClipToOutline(true);

        View scene = new View(this);
        GradientDrawable sceneGradient = new GradientDrawable(GradientDrawable.Orientation.TL_BR,
                new int[]{0xFF271711, 0xFF101821, 0xFF0A1017});
        sceneGradient.setCornerRadius(dp(22));
        scene.setBackground(sceneGradient);
        hero.addView(scene, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));

        View shade = new View(this);
        GradientDrawable gradient = new GradientDrawable(GradientDrawable.Orientation.LEFT_RIGHT,
                new int[]{0xE90A1017, 0xB20C1118, 0x350A0F15});
        gradient.setCornerRadius(dp(22));
        shade.setBackground(gradient);
        hero.addView(shade, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));

        LinearLayout copy = new LinearLayout(this);
        copy.setOrientation(LinearLayout.VERTICAL);
        copy.setPadding(dp(18), dp(19), dp(16), dp(14));
        TextView eyebrow = label("UŽIVO S BALKANA", 11, 0xFFFFB13D, true);
        copy.addView(eyebrow, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(26)));
        heroName = label("Radio Balkan", isCompactWidth() ? 27 : 30, Color.WHITE, true);
        copy.addView(heroName, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(44)));
        heroMeta = label("Odaberi stanicu i počni slušati", 14, 0xFFD2D4D8, false);
        copy.addView(heroMeta, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(30)));
        heroPlay = new Button(this);
        heroPlay.setText("▶   Slušaj uživo");
        heroPlay.setAllCaps(false);
        heroPlay.setTextColor(0xFF111217);
        heroPlay.setTextSize(15);
        heroPlay.setTypeface(Typeface.DEFAULT_BOLD);
        heroPlay.setPadding(0, 0, 0, 0);
        heroPlay.setMinWidth(0); heroPlay.setMinimumWidth(0); heroPlay.setMinHeight(0); heroPlay.setMinimumHeight(0);
        GradientDrawable playBg = new GradientDrawable(GradientDrawable.Orientation.LEFT_RIGHT, new int[]{0xFFFFB23F,0xFFFFC15B});
        playBg.setCornerRadius(dp(15));
        playBg.setStroke(dp(1), 0xFFFFC66F);
        heroPlay.setBackground(new RippleDrawable(ColorStateList.valueOf(0x33111111), playBg, rounded(Color.WHITE, 0x00000000, 15)));
        heroPlay.setEnabled(false);
        heroPlay.setContentDescription("Odaberi stanicu za reprodukciju");
        heroPlay.setOnClickListener(v -> { if (featured != null) onPlay(featured); });
        LinearLayout.LayoutParams buttonLp = new LinearLayout.LayoutParams(dp(176), dp(54));
        buttonLp.setMargins(0, dp(14), 0, 0);
        copy.addView(heroPlay, buttonLp);
        hero.addView(copy, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        return hero;
    }

    private View buildPlayerBar() {
        LinearLayout player = new LinearLayout(this);
        player.setOrientation(LinearLayout.HORIZONTAL);
        player.setGravity(Gravity.CENTER_VERTICAL);
        player.setPadding(dp(14), dp(10), dp(12), dp(10));
        GradientDrawable shell = new GradientDrawable(GradientDrawable.Orientation.LEFT_RIGHT, new int[]{0xFF111720,0xFF1A1716,0xFF0E131A});
        shell.setCornerRadius(dp(28));
        shell.setStroke(dp(1), 0xFF3B414C);
        player.setBackground(shell);

        boolean compactPlayer = isCompactWidth();
        if (!compactPlayer) {
            playerArtwork = new ImageView(this);
            playerArtwork.setScaleType(ImageView.ScaleType.CENTER_CROP);
            playerArtwork.setImageResource(R.drawable.ic_radio_balkan);
            playerArtwork.setBackground(rounded(0xFF20262F, 0xFF3C4653, 16));
            playerArtwork.setClipToOutline(true);
            player.addView(playerArtwork, new LinearLayout.LayoutParams(dp(68), dp(68)));
        } else {
            playerArtwork = null;
        }

        LinearLayout info = new LinearLayout(this);
        info.setOrientation(LinearLayout.VERTICAL);
        info.setPadding(dp(compactPlayer ? 6 : 12), 0, dp(compactPlayer ? 4 : 8), 0);
        TextView nowLabel = label("Sada svira", 11, 0xFFFFB23F, false);
        playerName = label("Odaberi radio stanicu", 17, Color.WHITE, true);
        playerMeta = label("Radio iz Hrvatske i regije", 11, 0xFFADB3BD, false);
        statusText = label("Spremno", 9, 0xFF788391, false);
        info.addView(nowLabel, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(21)));
        info.addView(playerName, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(27)));
        info.addView(playerMeta, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(21)));
        info.addView(statusText, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(17)));
        player.addView(info, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));

        playerEqualizer = new EqualizerView(this);
        playerEqualizer.setBarCount(9);
        if (!compactPlayer) {
            player.addView(playerEqualizer, new LinearLayout.LayoutParams(dp(58), dp(48)));
        }

        LinearLayout transport = new LinearLayout(this);
        transport.setGravity(Gravity.CENTER_VERTICAL);

        playerPrev = playerButton("◀", false);
        playerPrev.setContentDescription("Prethodna stanica");
        playerPrev.setOnClickListener(v -> playAdjacent(-1));
        transport.addView(playerPrev, new LinearLayout.LayoutParams(dp(40), dp(52)));

        playerPlay = playerButton("▶", true);
        GradientDrawable playBg = new GradientDrawable();
        playBg.setColor(0xFF201A16); playBg.setCornerRadius(dp(30)); playBg.setStroke(dp(2), 0xFFFFB23F);
        playerPlay.setBackground(new RippleDrawable(ColorStateList.valueOf(0x33FFFFFF), playBg, rounded(Color.WHITE, 0x00000000, 30)));
        playerPlay.setTextColor(0xFFFFF3DD);
        LinearLayout.LayoutParams pp = new LinearLayout.LayoutParams(dp(52), dp(56));
        pp.setMargins(dp(2), 0, dp(2), 0);
        transport.addView(playerPlay, pp);
        playerPlay.setEnabled(false);
        playerPlay.setContentDescription("Odaberi stanicu za reprodukciju");
        playerPlay.setOnClickListener(v -> {
            PlaybackLifecycle.UiCommand command = PlaybackLifecycle.uiCommand(!currentKey.isEmpty(), playing, playbackStopped);
            if (command == PlaybackLifecycle.UiCommand.PLAY) {
                RadioStation current = stationByKey(currentKey);
                if (current != null) { onPlay(current); return; }
                if (featured != null) { onPlay(featured); return; }
                return;
            }
            sendPlayerAction(command == PlaybackLifecycle.UiCommand.PAUSE
                    ? RadioPlayerService.ACTION_PAUSE
                    : RadioPlayerService.ACTION_RESUME);
        });

        playerStop = playerButton("■", false);
        playerStop.setContentDescription("Zaustavi reprodukciju");
        playerStop.setOnClickListener(v -> {
            if (currentKey.isEmpty() || playbackStopped) return;
            statusText.setText("Zaustavljam…");
            sendPlayerAction(RadioPlayerService.ACTION_STOP);
        });
        transport.addView(playerStop, new LinearLayout.LayoutParams(dp(40), dp(52)));

        playerNext = playerButton("▶", false);
        playerNext.setContentDescription("Sljedeća stanica");
        playerNext.setOnClickListener(v -> playAdjacent(1));
        transport.addView(playerNext, new LinearLayout.LayoutParams(dp(40), dp(52)));

        player.addView(transport, new LinearLayout.LayoutParams(dp(compactPlayer ? 176 : 180), ViewGroup.LayoutParams.MATCH_PARENT));

        return player;
    }

    private View buildBottomNav() {
        LinearLayout nav = new LinearLayout(this);
        nav.setGravity(Gravity.CENTER);
        nav.setPadding(dp(8), dp(6), dp(8), dp(5));
        nav.setBackground(rounded(0xFF0D1219, 0xFF303844, 0));
        nav.addView(navItem("⌂", "Početna", "all"), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));
        nav.addView(navItem("◇", "Otkrij", "discover"), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));
        nav.addView(navItem("▥", "Radio", "radio"), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));
        nav.addView(navItem("♡", "Omiljene", "favorites"), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));
        nav.addView(navItem("⋯", "Više", "more"), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));
        selectBottomNav(navSelection);
        return nav;
    }

    private View navItem(String icon, String title, String action) {
        LinearLayout item = new LinearLayout(this);
        item.setOrientation(LinearLayout.VERTICAL);
        item.setGravity(Gravity.CENTER);
        TextView ic = label(icon, 25, 0xFFB9C0CB, false); ic.setGravity(Gravity.CENTER);
        TextView tx = label(title, 11, 0xFFB9C0CB, false); tx.setGravity(Gravity.CENTER);
        item.addView(ic, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(36)));
        item.addView(tx, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(23)));
        bottomNavItems.put(action, item);
        item.setFocusable(true);
        item.setClickable(true);
        item.setContentDescription(title);
        item.setOnClickListener(v -> {
            if ("discover".equals(action)) { showBrowseDialog(); return; }
            if ("more".equals(action)) { showAppMenu(); return; }
            if ("radio".equals(action)) { showRadioLibrary(); return; }
            selectBottomNav(action);
            if ("all".equals(action)) { tab="all"; state.setTab(tab); applyFilterAsync(); list.smoothScrollToPosition(0); }
            else if ("favorites".equals(action)) { tab="favorites"; state.setTab(tab); applyFilterAsync(); }
        });
        updateBottomNavItem(action, item);
        return item;
    }

    private void showRadioLibrary() {
        tab = "all";
        state.setTab(tab);
        selectBottomNav("radio");
        applyFilterAsync();
        if (list != null) {
            list.post(() -> {
                if (destroyed || list.getCount() <= 1) return;
                list.smoothScrollToPosition(1);
            });
        }
    }

    private static String navSelectionForTab(String value) {
        if ("favorites".equals(value)) return "favorites";
        if ("all".equals(value)) return "all";
        return "radio";
    }

    private void selectBottomNav(String action) {
        navSelection = safe(action).isEmpty() ? "radio" : action;
        for (Map.Entry<String, LinearLayout> entry : bottomNavItems.entrySet()) updateBottomNavItem(entry.getKey(), entry.getValue());
    }

    private void updateBottomNavItem(String action, LinearLayout item) {
        if (item == null || item.getChildCount() < 2) return;
        boolean selected = action.equals(navSelection);
        item.setSelected(selected);
        item.setBackground(interactiveRounded(selected ? 0xFF382819 : 0x00000000, selected ? 0xFF5D4025 : 0x00000000, 14, 0x24FFFFFF));
        TextView icon = (TextView) item.getChildAt(0);
        TextView text = (TextView) item.getChildAt(1);
        int color = selected ? 0xFFFFB23F : 0xFFB9C0CB;
        icon.setTextColor(color);
        text.setTextColor(color);
        text.setTypeface(selected ? Typeface.DEFAULT_BOLD : Typeface.DEFAULT);
        item.setContentDescription(text.getText() + (selected ? ", odabrano" : ""));
    }

    private void updatePlaybackControls() {
        if (heroPlay != null) {
            boolean hasFeatured = featured != null;
            boolean featuredCurrent = hasFeatured && featured.key().equals(currentKey);
            heroPlay.setEnabled(hasFeatured);
            String heroAction;
            if (featuredCurrent && playing) heroAction = "Ⅱ   Pauziraj";
            else if (featuredCurrent && playbackStopped) heroAction = "▶   Pokreni";
            else if (featuredCurrent && !currentKey.isEmpty()) heroAction = "▶   Nastavi";
            else heroAction = "▶   Slušaj uživo";
            heroPlay.setText(heroAction);
            heroPlay.setContentDescription(hasFeatured ? heroAction.replace("▶", "").replace("Ⅱ", "").trim() + " " + featured.name : "Odaberi stanicu za reprodukciju");
        }
        if (playerPlay != null) {
            boolean available = !currentKey.isEmpty() || featured != null;
            playerPlay.setEnabled(available);
            playerPlay.setText(playing ? "Ⅱ" : "▶");
            playerPlay.setContentDescription(available ? (playing ? "Pauziraj reprodukciju" : "Pokreni reprodukciju") : "Odaberi stanicu za reprodukciju");
        }
        int navigationCount = visibleStations.size();
        if (navigationCount == 0) {
            synchronized (dataLock) { navigationCount = allStations.size(); }
        }
        boolean canNavigate = navigationCount > 1;
        if (playerPrev != null) playerPrev.setEnabled(canNavigate);
        if (playerNext != null) playerNext.setEnabled(canNavigate);
        if (playerStop != null) {
            boolean canStop = !currentKey.isEmpty() && !playbackStopped;
            playerStop.setEnabled(canStop);
            playerStop.setContentDescription(canStop ? "Zaustavi reprodukciju" : "Reprodukcija je zaustavljena");
        }
    }

    private FrameLayout.LayoutParams bottomNavLayoutParams() {
        FrameLayout.LayoutParams p = new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(NAV_H_DP));
        p.gravity = Gravity.BOTTOM;
        return p;
    }

    private void showAppMenu() {
        String[] items = {"Pretraži", "Filtriraj stanice", "Poništi filtre", "Osvježi popis", "Provjeri prikazane stanice", "Rezervni izvori", "Nedostupne stanice", "O aplikaciji"};
        new AlertDialog.Builder(this).setTitle("Radio Balkan").setItems(items, (d, which) -> {
            String chosen = items[which];
            if ("Pretraži".equals(chosen)) {
                setSearchVisible(true);
            } else if ("Filtriraj stanice".equals(chosen)) {
                showBrowseDialog();
            } else if ("Poništi filtre".equals(chosen)) {
                resetBrowseFilters();
            } else if ("Osvježi popis".equals(chosen)) {
                refreshCatalog();
            } else if ("Provjeri prikazane stanice".equals(chosen)) {
                checkVisibleStreams();
            } else if ("Rezervni izvori".equals(chosen)) {
                tab = "replaced"; state.setTab(tab); selectBottomNav("radio"); applyFilterAsync();
            } else if ("Nedostupne stanice".equals(chosen)) {
                tab = "broken"; state.setTab(tab); selectBottomNav("radio"); applyFilterAsync();
            } else {
                showAboutDialog();
            }
        }).show();
    }

    private void showBrowseDialog() {
        LinearLayout panel = new LinearLayout(this);
        panel.setOrientation(LinearLayout.VERTICAL);
        panel.setPadding(dp(8), dp(4), dp(8), dp(4));

        TextView cTitle = label("Država", 14, Color.WHITE, true);
        panel.addView(cTitle, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(32)));
        LinearLayout countries = chipRow();
        List<Button> countryButtons = new ArrayList<>();
        Map<String,Integer> counts = countryCounts();
        int total = 0;
        for (int v : counts.values()) total += v;
        for (String[] c : RadioRepository.COUNTRIES) {
            String code = c[0], name = c[1];
            int count = code.isEmpty() ? total : counts.getOrDefault(code, 0);
            Button b = chip(count > 0 ? name + " · " + count : name, code.equalsIgnoreCase(country));
            b.setTag(code);
            applyFlag(b, code);
            countryButtons.add(b);
            b.setOnClickListener(v -> {
                country = code;
                state.setCountry(code);
                for (Button item : countryButtons) styleChip(item, String.valueOf(item.getTag()).equalsIgnoreCase(country));
                applyFilterAsync();
            });
            countries.addView(b, chipParams());
        }
        panel.addView(horizontal(countries), new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(48)));

        TextView gTitle = label("Žanr", 14, Color.WHITE, true);
        panel.addView(gTitle, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(32)));
        LinearLayout genres = chipRow();
        List<Button> genreButtons = new ArrayList<>();
        String[][] gs = {{"Svi",""},{"Domaća","domaca"},{"Pop & Rock","pop"},{"Narodna","folk"},{"Elektronička","electronic"},{"Jazz","jazz"},{"Klasična","classical"}};
        for (String[] g : gs) {
            String label = g[0], value = g[1];
            Button b = chip(label, value.equalsIgnoreCase(genre));
            b.setTag(value);
            genreButtons.add(b);
            b.setOnClickListener(v -> {
                genre = value;
                state.setGenre(value);
                for (Button item : genreButtons) styleChip(item, String.valueOf(item.getTag()).equalsIgnoreCase(genre));
                applyFilterAsync();
            });
            genres.addView(b, chipParams());
        }
        panel.addView(horizontal(genres), new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(48)));

        new AlertDialog.Builder(this)
                .setTitle("Pregledaj stanice")
                .setView(panel)
                .setNeutralButton("Poništi filtre", (d, which) -> resetBrowseFilters())
                .setPositiveButton("Gotovo", null)
                .show();
    }

    private void resetBrowseFilters() {
        country = "";
        genre = "";
        tab = "all";
        query = "";
        state.setCountry("");
        state.setGenre("");
        state.setTab("all");
        if (search != null && search.getText().length() > 0) search.setText("");
        selectBottomNav("all");
        statusText.setText("Filtri su poništeni");
        applyFilterAsync();
    }

    private void showAboutDialog() {
        new AlertDialog.Builder(this).setTitle("Radio Balkan").setMessage("Radio Balkan " + AppInfo.VERSION + "\n\nRadio stanice samo iz Hrvatske, Bosne i Hercegovine, Srbije, Slovenije, Sjeverne Makedonije, Albanije i Crne Gore. Favoriti, povijest slušanja, provjera dostupnosti i zamjenski izvori ostaju lokalno na uređaju.").setPositiveButton("U redu", null).show();
    }

    private void loadStations() {
        final int gen = catalogGeneration.incrementAndGet();
        statusText.setText("Učitavam radio stanice…");
        RadioRepository repo = repository;
        repo.load(new RadioRepository.Listener() {
            @Override public void onCached(List<RadioStation> stations) { postIfActive(() -> { if (gen == catalogGeneration.get()) useStations(stations, "Spremljeni popis · osvježavam…"); }); }
            @Override public void onLoaded(List<RadioStation> stations) { postIfActive(() -> { if (gen == catalogGeneration.get()) useStations(stations, "Ažurirano"); }); }
            @Override public void onError(Throwable error, boolean hasCache) { postIfActive(() -> { if (gen == catalogGeneration.get()) statusText.setText(hasCache ? "Nema mreže · koristi se spremljeni popis" : "Popis trenutno nije dostupan"); }); }
        });
    }

    private void useStations(List<RadioStation> stations, String status) {
        if (destroyed || stations == null) return;
        Map<String, RadioStation> old = new HashMap<>();
        synchronized (dataLock) { for (RadioStation s : allStations) old.put(s.key(), s); }
        for (RadioStation s : stations) {
            String key = s.key();
            String manual = state.manualReplacement(key);
            String automatic = state.autoReplacement(key);
            if (!manual.isEmpty()) { s.activeUrl = manual; s.replaced = true; }
            else if (!automatic.isEmpty()) { s.activeUrl = automatic; s.replaced = true; }
            RadioStation previous = old.get(key);
            if (previous != null) {
                if (!"unknown".equals(previous.health)) s.health = previous.health;
                if (manual.isEmpty() && automatic.isEmpty() && StreamResolver.isHttp(previous.activeUrl)) s.activeUrl = previous.activeUrl;
                s.replaced = s.replaced || previous.replaced;
            }
            s.refreshIndexes();
        }
        synchronized (dataLock) { allStations = new ArrayList<>(stations); }
        statusText.setText(status);
        applyFilterAsync();
        scheduleAutomaticHealthScan();
    }

    private void refreshCatalog() {
        if (destroyed) return;
        if (!catalogRefreshRunning.compareAndSet(false, true)) {
            Toast.makeText(this, "Osvježavanje je već u tijeku", Toast.LENGTH_SHORT).show();
            return;
        }
        statusText.setText("Osvježavam popis stanica…");
        int gen = catalogGeneration.incrementAndGet();
        RadioRepository old = repository;
        repository = new RadioRepository(this);
        old.shutdown();
        repository.load(new RadioRepository.Listener() {
            @Override public void onCached(List<RadioStation> stations) { }
            @Override public void onLoaded(List<RadioStation> stations) {
                catalogRefreshRunning.set(false);
                postIfActive(() -> { if (gen != catalogGeneration.get()) return; useStations(stations, "Popis je osvježen"); });
            }
            @Override public void onError(Throwable error, boolean hasCache) {
                catalogRefreshRunning.set(false);
                postIfActive(() -> { if (gen != catalogGeneration.get()) return; statusText.setText("Osvježavanje nije uspjelo"); });
            }
        });
    }

    private void applyFilterAsync() {
        final int generation = filterGeneration.incrementAndGet();
        final String q = query, c = country, g = genre, activeTab = tab;
        final List<RadioStation> source;
        synchronized (dataLock) { source = allStations; }
        final Set<String> favorites = state.favorites();
        final List<String> recentList = state.recent();
        final Set<String> recent = new HashSet<>(recentList);
        try {
            filterWorker.execute(() -> {
                List<RadioStation> out = new ArrayList<>();
                for (RadioStation s : source) {
                    if (!c.isEmpty() && !c.equalsIgnoreCase(s.countryCode)) continue;
                    if (!q.isEmpty() && !s.searchIndex.contains(q)) continue;
                    if (!g.isEmpty() && !matchesGenre(s, g)) continue;
                    if ("favorites".equals(activeTab) && !favorites.contains(s.key())) continue;
                    if ("recent".equals(activeTab) && !recent.contains(s.key())) continue;
                    if ("popular".equals(activeTab) && s.votes <= 0) continue;
                    if ("broken".equals(activeTab) && !"broken".equals(s.health)) continue;
                    if ("replaced".equals(activeTab) && !s.replaced) continue;
                    out.add(s);
                }
                if ("recent".equals(activeTab)) {
                    Map<String, Integer> order = new HashMap<>();
                    for (int i = 0; i < recentList.size(); i++) order.put(recentList.get(i), i);
                    out.sort(Comparator.comparingInt(x -> order.getOrDefault(x.key(), Integer.MAX_VALUE)));
                } else {
                    out.sort(Comparator.comparingInt((RadioStation s) -> s.votes).reversed().thenComparing(s -> s.name.toLowerCase(Locale.ROOT)));
                }
                if (generation != filterGeneration.get() || destroyed) return;
                postIfActive(() -> {
                    if (generation != filterGeneration.get()) return;
                    visibleStations = out;
                    adapter.setSnapshot(out, currentKey, playing);
                    featured = out.isEmpty() ? null : out.get(0);
                    if (featured == null) {
                        heroName.setText("Nema rezultata");
                        heroMeta.setText("Promijeni filter ili pretragu");
                        clearFlag(heroMeta);
                        heroPlay.setEnabled(false);
                    } else {
                        heroName.setText(featured.name);
                        heroMeta.setText(featured.meta());
                        applyFlag(heroMeta, featured.countryCode);
                    }
                    updatePlaybackControls();
                });
            });
        } catch (RejectedExecutionException ignored) { }
    }

    private Map<String, Integer> countryCounts() {
        Map<String, Integer> out = new HashMap<>();
        synchronized (dataLock) {
            for (RadioStation station : allStations) {
                String code = safe(station.countryCode).toUpperCase(Locale.ROOT);
                if (!code.isEmpty()) out.put(code, out.getOrDefault(code, 0) + 1);
            }
        }
        return out;
    }

    private boolean matchesGenre(RadioStation s, String g) {
        String t = s.searchIndex;
        switch (g) {
            case "domaca": return containsAny(t, "domaca", "ex yu", "ex-yu", "balkan", "regional", "croatian", "hrvatska");
            case "pop": return containsAny(t, "pop", "rock", "indie", "alternative");
            case "folk": return containsAny(t, "folk", "narodna", "narodno", "turbo folk", "sevdah", "sevdalinka", "etno", "krajiska");
            case "electronic": return containsAny(t, "electronic", "dance", "house", "techno", "edm", "trance", "club");
            case "classical": return containsAny(t, "classical", "klasicna");
            default: return t.contains(RadioStation.fold(g));
        }
    }

    private boolean containsAny(String value, String... needles) { for (String n : needles) if (value.contains(RadioStation.fold(n))) return true; return false; }

    private void scheduleAutomaticHealthScan() {
        if (!autoHealthStarted.compareAndSet(false, true)) return;
        ui.postDelayed(() -> {
            if (destroyed) return;
            List<RadioStation> source;
            synchronized (dataLock) { source = new ArrayList<>(allStations); }
            Set<String> fav = state.favorites(); Set<String> recent = new HashSet<>(state.recent());
            source.sort((a, b) -> {
                int pa = fav.contains(a.key()) ? 0 : recent.contains(a.key()) ? 1 : 2;
                int pb = fav.contains(b.key()) ? 0 : recent.contains(b.key()) ? 1 : 2;
                if (pa != pb) return Integer.compare(pa, pb);
                return Integer.compare(b.votes, a.votes);
            });
            if (source.size() > 16) source = new ArrayList<>(source.subList(0, 16));
            startHealthScan(source, false);
        }, 8000);
    }

    private void checkVisibleStreams() {
        List<RadioStation> targets = new ArrayList<>(visibleStations);
        if (targets.isEmpty()) { Toast.makeText(this, "Nema stanica za provjeru", Toast.LENGTH_SHORT).show(); return; }
        startHealthScan(targets, true);
    }

    private void startHealthScan(List<RadioStation> targets, boolean userInitiated) {
        if (targets == null || targets.isEmpty() || destroyed) return;
        if (!healthRunning.compareAndSet(false, true)) {
            if (userInitiated) Toast.makeText(this, "Provjera je već u tijeku", Toast.LENGTH_SHORT).show();
            return;
        }
        final int generation = healthGeneration.incrementAndGet();
        LinkedHashMap<String, RadioStation> uniqueMap = new LinkedHashMap<>();
        for (RadioStation station : targets) uniqueMap.put(station.key(), station);
        final List<RadioStation> unique = new ArrayList<>(uniqueMap.values());
        if (userInitiated) statusText.setText("Provjeravam dostupnost 0/" + unique.size() + "…");
        AtomicInteger next = new AtomicInteger(); AtomicInteger done = new AtomicInteger();
        int workers = Math.min(userInitiated ? 2 : 1, unique.size());
        for (int w = 0; w < workers; w++) {
            try {
                ioWorker.execute(() -> {
                    for (;;) {
                        if (destroyed || generation != healthGeneration.get()) break;
                        int i = next.getAndIncrement();
                        if (i >= unique.size()) break;
                        RadioStation station = unique.get(i);
                        try {
                            station.health = "checking";
                            StreamResolver.Resolution result = StreamResolver.repair(StreamResolver.candidates(station, state), station.stationUuid, station.homepage, station.countryCode);
                            if (result != null) {
                                station.health = "ok";
                                station.activeUrl = result.url;
                                state.addBackup(station.key(), result.url);
                                String manual = state.manualReplacement(station.key());
                                if (manual.isEmpty() && !sameUrl(result.url, station.urlResolved) && !sameUrl(result.url, station.url)) {
                                    state.setAutoReplacement(station.key(), result.url);
                                    station.replaced = true;
                                }
                            } else {
                                station.health = "broken";
                            }
                        } catch (Throwable error) {
                            station.health = "unknown";
                            AppLog.e(MainActivity.this, "health-" + station.key(), error);
                        } finally {
                            int finished = done.incrementAndGet();
                            if (finished % 8 == 0 || finished == unique.size()) postIfActive(() -> {
                                adapter.notifyDataSetChanged();
                                if (userInitiated) statusText.setText("Provjera " + finished + "/" + unique.size() + "…");
                            });
                        }
                    }
                    if (done.get() >= unique.size() && healthRunning.compareAndSet(true, false)) postIfActive(() -> {
                        if (userInitiated) statusText.setText("Provjera završena");
                        adapter.notifyDataSetChanged();
                        if ("broken".equals(tab) || "replaced".equals(tab)) applyFilterAsync();
                    });
                });
            } catch (RejectedExecutionException ignored) { healthRunning.set(false); }
        }
    }

    private void checkOne(RadioStation s) {
        if (s == null || destroyed) return;
        statusText.setText("Provjeravam · " + s.name);
        try {
            ioWorker.execute(() -> {
                try {
                    s.health = "checking"; postIfActive(adapter::notifyDataSetChanged);
                    StreamResolver.Resolution result = StreamResolver.repair(StreamResolver.candidates(s, state), s.stationUuid, s.homepage, s.countryCode);
                    if (result != null) {
                        s.health = "ok"; s.activeUrl = result.url; state.addBackup(s.key(), result.url);
                        if (state.manualReplacement(s.key()).isEmpty() && !sameUrl(result.url, s.urlResolved) && !sameUrl(result.url, s.url)) { state.setAutoReplacement(s.key(), result.url); s.replaced = true; }
                        postIfActive(() -> { statusText.setText("Dostupno · " + s.name); adapter.notifyDataSetChanged(); });
                    } else {
                        s.health = "broken";
                        postIfActive(() -> { statusText.setText("Nedostupno · " + s.name); adapter.notifyDataSetChanged(); });
                    }
                } catch (Throwable error) {
                    s.health = "unknown";
                    AppLog.e(MainActivity.this, "health-one-" + s.key(), error);
                    postIfActive(() -> { statusText.setText("Provjera nije uspjela · " + s.name); adapter.notifyDataSetChanged(); });
                }
            });
        } catch (RejectedExecutionException ignored) { }
    }

    @Override public void onPlay(RadioStation s) {
        if (s == null) return;
        hideKeyboard();
        requestNotificationPermission();
        String key = s.key();
        PlaybackLifecycle.UiCommand command = PlaybackLifecycle.uiCommand(key.equals(currentKey), playing, playbackStopped);
        if (command != PlaybackLifecycle.UiCommand.PLAY) {
            sendPlayerAction(command == PlaybackLifecycle.UiCommand.PAUSE
                    ? RadioPlayerService.ACTION_PAUSE
                    : RadioPlayerService.ACTION_RESUME);
            return;
        }
        Intent i = new Intent(this, RadioPlayerService.class).setAction(RadioPlayerService.ACTION_PLAY);
        i.putExtra(RadioPlayerService.EXTRA_KEY, key); i.putExtra(RadioPlayerService.EXTRA_NAME, s.name); i.putExtra(RadioPlayerService.EXTRA_META, s.meta());
        i.putExtra(RadioPlayerService.EXTRA_URL, s.url); i.putExtra(RadioPlayerService.EXTRA_RESOLVED, s.urlResolved); i.putExtra(RadioPlayerService.EXTRA_UUID, s.stationUuid); i.putExtra(RadioPlayerService.EXTRA_HOMEPAGE, s.homepage); i.putExtra(RadioPlayerService.EXTRA_COUNTRY, s.countryCode);
        try { if (Build.VERSION.SDK_INT >= 26) startForegroundService(i); else startService(i); }
        catch (Throwable t) { AppLog.e(this, "player-play", t); Toast.makeText(this, "Reprodukciju nije moguće pokrenuti", Toast.LENGTH_LONG).show(); return; }
        currentKey = key;
        state.addRecent(key);
        state.setLastStation(key, s.name, s.meta());
        currentCountryCode = s.countryCode;
        playerName.setText(s.name); playerMeta.setText(s.meta()); applyFlag(playerMeta, s.countryCode); statusText.setText("Povezujem…");
        if (playerArtwork != null) { playerArtwork.setImageResource(R.drawable.ic_radio_balkan); images.load(s.favicon, playerArtwork, null); }
        if (playerEqualizer != null) playerEqualizer.setActive(false);
        playing = false;
        playbackStopped = false;
        updatePlaybackControls();
        adapter.setPlayback(currentKey, false);
    }

    private void playAdjacent(int delta) {
        if (delta == 0) return;
        List<RadioStation> source = new ArrayList<>(visibleStations);
        if (source.isEmpty()) {
            synchronized (dataLock) { source = new ArrayList<>(allStations); }
        }
        if (source.isEmpty()) {
            Toast.makeText(this, "Nema stanica za reprodukciju", Toast.LENGTH_SHORT).show();
            return;
        }
        int currentIndex = -1;
        for (int i = 0; i < source.size(); i++) {
            if (source.get(i).key().equals(currentKey)) {
                currentIndex = i;
                break;
            }
        }
        int nextIndex = PlaybackLifecycle.adjacentIndex(source.size(), currentIndex, delta);
        if (nextIndex >= 0 && nextIndex < source.size()) onPlay(source.get(nextIndex));
    }

    @Override public void onFavorite(RadioStation s) {
        boolean on = state.toggleFavorite(s.key());
        Toast.makeText(this, on ? "Dodano u omiljene" : "Uklonjeno iz omiljenih", Toast.LENGTH_SHORT).show();
        if ("favorites".equals(tab)) applyFilterAsync();
    }

    @Override public void onMore(RadioStation s) {
        List<String> options = new ArrayList<>();
        options.add("▶ Slušaj");
        options.add(state.favorites().contains(s.key()) ? "Ukloni iz omiljenih" : "Dodaj u omiljene");
        if (StreamResolver.isHttp(s.homepage)) options.add("Web stranica");
        options.add("Kopiraj poveznicu za reprodukciju");
        options.add("Odaberi drugi izvor");
        options.add("Provjeri dostupnost");
        if (state.manualReplacement(s.key()).isEmpty() && (!state.autoReplacement(s.key()).isEmpty() || !state.backups(s.key()).isEmpty())) options.add("Vrati automatski odabir");
        String[] array = options.toArray(new String[0]);
        new AlertDialog.Builder(this).setTitle(s.name).setItems(array, (d, which) -> {
            String chosen = array[which];
            if (chosen.startsWith("▶")) onPlay(s);
            else if (chosen.equals("Dodaj u omiljene") || chosen.equals("Ukloni iz omiljenih")) onFavorite(s);
            else if (chosen.equals("Web stranica")) openWeb(s);
            else if (chosen.equals("Kopiraj poveznicu za reprodukciju")) copyText(s.activeUrl);
            else if (chosen.equals("Odaberi drugi izvor")) showSourceDialog(s);
            else if (chosen.equals("Provjeri dostupnost")) checkOne(s);
            else if (chosen.equals("Vrati automatski odabir")) {
                state.clearAutomaticSources(s.key());
                String manual = state.manualReplacement(s.key());
                s.replaced = !manual.isEmpty();
                s.activeUrl = !manual.isEmpty() ? manual : (!s.urlResolved.isEmpty() ? s.urlResolved : s.url);
                adapter.notifyDataSetChanged();
                statusText.setText(manual.isEmpty() ? "Vraćen automatski odabir izvora" : "Ručni izvor ostaje aktivan");
            }
        }).setNegativeButton("Zatvori", null).show();
    }

    private void openWeb(RadioStation s) {
        if (!StreamResolver.isSafeHttp(s.homepage)) { Toast.makeText(this, "Web stranica nije dostupna", Toast.LENGTH_SHORT).show(); return; }
        try { startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse(s.homepage))); }
        catch (Throwable t) { AppLog.e(this, "open-web", t); Toast.makeText(this, "Nije moguće otvoriti web stranicu", Toast.LENGTH_SHORT).show(); }
    }

    private void showSourceDialog(RadioStation s) {
        final EditText input = new EditText(this); input.setSingleLine(true);
        String existing = state.manualReplacement(s.key()); input.setText(existing.isEmpty() ? s.activeUrl : existing);
        input.setTextColor(Color.WHITE); input.setHintTextColor(0xFF85756D); input.setHint("https://stream…"); input.setBackground(rounded(0xFF2A211D, 0xFF4B3A31, 10)); input.setPadding(dp(12), 0, dp(12), 0);
        LinearLayout wrap = new LinearLayout(this); wrap.setPadding(dp(20), dp(8), dp(20), 0); wrap.addView(input, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(52)));
        AlertDialog dialog = new AlertDialog.Builder(this)
                .setTitle("Izvor reprodukcije · " + s.name)
                .setMessage("Unesi izravnu http/https poveznicu za reprodukciju. Prazno polje vraća automatski odabir.")
                .setView(wrap).setPositiveButton("Spremi", null).setNeutralButton("Kopiraj trenutačnu", null).setNegativeButton("Odustani", null).create();
        dialog.setOnShowListener(x -> {
            dialog.getButton(AlertDialog.BUTTON_POSITIVE).setOnClickListener(v -> {
                String value = input.getText().toString().trim();
                if (value.isEmpty()) {
                    state.setManualReplacement(s.key(), "");
                    s.replaced = !state.autoReplacement(s.key()).isEmpty();
                    s.activeUrl = !state.autoReplacement(s.key()).isEmpty() ? state.autoReplacement(s.key()) : (!s.urlResolved.isEmpty() ? s.urlResolved : s.url);
                    adapter.notifyDataSetChanged();
                    dialog.dismiss();
                    return;
                }
                if (!StreamResolver.isSafeHttp(value)) { input.setError("Unesi sigurnu javnu http/https poveznicu"); return; }
                Button save = dialog.getButton(AlertDialog.BUTTON_POSITIVE);
                save.setEnabled(false);
                save.setText("Provjeravam…");
                input.setEnabled(false);
                try {
                    ioWorker.execute(() -> {
                        String resolved = null;
                        Throwable failure = null;
                        try {
                            resolved = StreamResolver.resolveFirst(java.util.Collections.singletonList(value));
                        } catch (Throwable error) {
                            failure = error;
                            AppLog.e(MainActivity.this, "source-check-" + s.key(), error);
                        }
                        final String checkedUrl = resolved;
                        final boolean checkFailed = failure != null;
                        postIfActive(() -> {
                            if (!dialog.isShowing()) return;
                            input.setEnabled(true);
                            save.setEnabled(true);
                            save.setText("Spremi");
                            if (checkFailed) {
                                input.setError("Provjera trenutačno nije dostupna. Pokušaj ponovno.");
                                return;
                            }
                            if (checkedUrl == null || checkedUrl.isEmpty()) {
                                input.setError("Izvor nije dostupan. Provjeri URL i pokušaj ponovno.");
                                return;
                            }
                            state.setManualReplacement(s.key(), checkedUrl);
                            state.addBackup(s.key(), checkedUrl);
                            s.replaced = true;
                            s.activeUrl = checkedUrl;
                            s.health = "ok";
                            adapter.notifyDataSetChanged();
                            statusText.setText("Izvor je provjeren i spremljen");
                            dialog.dismiss();
                        });
                    });
                } catch (RejectedExecutionException error) {
                    save.setEnabled(true); save.setText("Spremi"); input.setEnabled(true);
                    input.setError("Provjera trenutačno nije dostupna");
                }
            });
            dialog.getButton(AlertDialog.BUTTON_NEUTRAL).setOnClickListener(v -> copyText(s.activeUrl));
        });
        dialog.show();
    }

    private void sendPlayerAction(String action) {
        Intent i = new Intent(this, RadioPlayerService.class).setAction(action);
        try { startService(i); } catch (Throwable t) { AppLog.e(this, "player-action", t); Toast.makeText(this, "Radnja trenutačno nije dostupna", Toast.LENGTH_SHORT).show(); }
    }

    private void queryPlayerState() {
        try { startService(new Intent(this, RadioPlayerService.class).setAction(RadioPlayerService.ACTION_QUERY)); }
        catch (Throwable t) { AppLog.e(this, "player-query", t); }
    }

    // Pre-33 registration is protected by INTERNAL_STATE_PERMISSION; lint cannot infer that custom signature permission.
    @android.annotation.SuppressLint("UnspecifiedRegisterReceiverFlag")
    private void registerPlayerReceiver() {
        try {
            IntentFilter f = new IntentFilter(RadioPlayerService.ACTION_STATE);
            if (Build.VERSION.SDK_INT >= 33) {
                registerReceiver(playerReceiver, f, RadioPlayerService.INTERNAL_STATE_PERMISSION, null, Context.RECEIVER_NOT_EXPORTED);
            } else {
                registerReceiver(playerReceiver, f, RadioPlayerService.INTERNAL_STATE_PERMISSION, null);
            }
            receiverRegistered = true;
        } catch (Throwable t) {
            AppLog.e(this, "player-receiver", t);
            receiverRegistered = false;
        }
    }

    private void requestNotificationPermission() {
        try {
            if (Build.VERSION.SDK_INT >= 33 && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                requestPermissions(new String[]{Manifest.permission.POST_NOTIFICATIONS}, 1001);
            }
        } catch (Throwable t) {
            AppLog.e(this, "notification-permission", t);
        }
    }

    private void copyText(String value) {
        if (value == null || value.trim().isEmpty()) { Toast.makeText(this, "Izvor nije dostupan", Toast.LENGTH_SHORT).show(); return; }
        ClipboardManager cm = (ClipboardManager) getSystemService(CLIPBOARD_SERVICE);
        if (cm == null) { Toast.makeText(this, "Međuspremnik nije dostupan", Toast.LENGTH_SHORT).show(); return; }
        cm.setPrimaryClip(ClipData.newPlainText("Radio Balkan", value));
        Toast.makeText(this, "Kopirano", Toast.LENGTH_SHORT).show();
    }

    private void hideKeyboard() {
        View v = getCurrentFocus();
        if (v != null) { InputMethodManager im = (InputMethodManager) getSystemService(INPUT_METHOD_SERVICE); if (im != null) im.hideSoftInputFromWindow(v.getWindowToken(), 0); }
    }

    @SuppressWarnings("deprecation")
    private void installSystemBarInsets(View root) {
        root.setOnApplyWindowInsetsListener((v, insets) -> {
            int top;
            int bottom;
            if (Build.VERSION.SDK_INT >= 30) {
                android.graphics.Insets bars = insets.getInsets(WindowInsets.Type.systemBars());
                top = bars.top;
                bottom = bars.bottom;
            } else {
                top = insets.getSystemWindowInsetTop();
                bottom = insets.getSystemWindowInsetBottom();
            }
            v.setPadding(0, top, 0, bottom);
            return insets;
        });
        root.requestApplyInsets();
    }

    private boolean isCompactWidth() {
        float density = getResources().getDisplayMetrics().density;
        return getResources().getDisplayMetrics().widthPixels / Math.max(1f, density) < 390f;
    }

    private void postIfActive(Runnable r) {
        if (destroyed || r == null) return;
        ui.post(() -> {
            if (destroyed || isFinishing()) return;
            try {
                r.run();
            } catch (Throwable error) {
                AppLog.e(MainActivity.this, "ui-update", error);
                if (statusText != null) statusText.setText("Prikaz nije moguće osvježiti");
            }
        });
    }

    private void setSearchVisible(boolean show) {
        if (searchBox == null || search == null) return;
        searchBox.setVisibility(show ? View.VISIBLE : View.GONE);
        InputMethodManager imm = (InputMethodManager) getSystemService(INPUT_METHOD_SERVICE);
        if (show) {
            search.requestFocus();
            if (imm != null) imm.showSoftInput(search, InputMethodManager.SHOW_IMPLICIT);
            return;
        }

        search.clearFocus();
        if (imm != null) imm.hideSoftInputFromWindow(search.getWindowToken(), 0);
        if (searchRunnable != null) ui.removeCallbacks(searchRunnable);
        searchRunnable = null;
        if (search.length() > 0) search.setText("");
        query = "";
        applyFilterAsync();
    }

    private void installBackHandler() {
        if (Build.VERSION.SDK_INT >= 33) {
            backInvokedCallback = this::handleBackNavigation;
            getOnBackInvokedDispatcher().registerOnBackInvokedCallback(
                    android.window.OnBackInvokedDispatcher.PRIORITY_DEFAULT,
                    backInvokedCallback);
        }
    }

    private void uninstallBackHandler() {
        if (Build.VERSION.SDK_INT >= 33 && backInvokedCallback != null) {
            try {
                getOnBackInvokedDispatcher().unregisterOnBackInvokedCallback(backInvokedCallback);
            } catch (Throwable error) {
                AppLog.e(this, "back-handler-unregister", error);
            }
            backInvokedCallback = null;
        }
    }

    private boolean closeTransientUiForBack() {
        if (searchBox != null && searchBox.getVisibility() == View.VISIBLE) {
            setSearchVisible(false);
            return true;
        }
        return false;
    }

    private void handleBackNavigation() {
        if (closeTransientUiForBack()) return;
        finishAfterTransition();
    }

    @SuppressWarnings("deprecation")
    @Override public void onBackPressed() {
        if (Build.VERSION.SDK_INT >= 33) {
            handleBackNavigation();
            return;
        }
        if (closeTransientUiForBack()) return;
        super.onBackPressed();
    }

    private static boolean sameUrl(String a, String b) { return safe(a).equalsIgnoreCase(safe(b)); }
    private static boolean validTab(String value) { return "all".equals(value) || "popular".equals(value) || "favorites".equals(value) || "recent".equals(value) || "replaced".equals(value) || "broken".equals(value); }
    private RadioStation stationByKey(String key) {
        if (key == null || key.isEmpty()) return null;
        synchronized (dataLock) {
            for (RadioStation station : allStations) if (key.equals(station.key())) return station;
        }
        return null;
    }

    private void applyFlag(TextView view, String code) {
        if (view == null) return;
        CountryFlagDrawable flag = new CountryFlagDrawable(code);
        flag.setBounds(0, 0, dp(24), dp(15));
        view.setCompoundDrawablePadding(dp(6));
        view.setCompoundDrawables(flag, null, null, null);
    }

    private void clearFlag(TextView view) {
        if (view == null) return;
        view.setCompoundDrawables(null, null, null, null);
        view.setCompoundDrawablePadding(0);
    }

    private LinearLayout chipRow() { LinearLayout l = new LinearLayout(this); l.setOrientation(LinearLayout.HORIZONTAL); l.setGravity(Gravity.CENTER_VERTICAL); return l; }
    private HorizontalScrollView horizontal(View child) { HorizontalScrollView h = new HorizontalScrollView(this); h.setHorizontalScrollBarEnabled(false); h.setFillViewport(false); h.addView(child); return h; }
    private Button chip(String label, boolean selected) {
        Button b = new Button(this); b.setText(label); b.setAllCaps(false); b.setTextSize(11);
        b.setPadding(dp(12), 0, dp(12), 0); b.setMinHeight(0); b.setMinimumHeight(0); b.setMinWidth(0); b.setMinimumWidth(0);
        styleChip(b, selected);
        return b;
    }
    private void styleChip(Button b, boolean selected) {
        if (b == null) return;
        b.setTypeface(selected ? Typeface.DEFAULT_BOLD : Typeface.DEFAULT);
        b.setTextColor(selected ? 0xFFFFFFFF : 0xFFE4DBD5);
        b.setBackground(interactiveRounded(selected ? 0xFFEB5323 : 0xFF2A211D, selected ? 0xFFEB5323 : 0xFF4B3A31, 14, 0x30FFFFFF));
    }
    private LinearLayout.LayoutParams chipParams() { LinearLayout.LayoutParams p = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, dp(40)); p.setMargins(dp(3), dp(4), dp(4), dp(3)); return p; }
    private Button smallTop(String label) { Button b = chip(label, false); b.setTextSize(11); return b; }
    private Button playerButton(String label, boolean accent) { Button b = chip(label, accent); b.setTextSize(17); return b; }
    private TextView label(String value, int size, int color, boolean bold) { TextView t = new TextView(this); t.setText(value); t.setTextSize(size); t.setTextColor(color); t.setGravity(Gravity.CENTER_VERTICAL); if (bold) t.setTypeface(Typeface.DEFAULT_BOLD); t.setSingleLine(true); t.setEllipsize(android.text.TextUtils.TruncateAt.END); return t; }
    private GradientDrawable rounded(int fill, int stroke, int radiusDp) { GradientDrawable g = new GradientDrawable(); g.setColor(fill); g.setCornerRadius(dp(radiusDp)); g.setStroke(dp(1), stroke); return g; }
    private android.graphics.drawable.Drawable interactiveRounded(int fill, int stroke, int radiusDp, int rippleColor) {
        GradientDrawable content = rounded(fill, stroke, radiusDp);
        GradientDrawable mask = rounded(Color.WHITE, 0x00000000, radiusDp);
        return new RippleDrawable(ColorStateList.valueOf(rippleColor), content, mask);
    }
    private LinearLayout.LayoutParams marginParams(int w, int h, int l, int t, int r, int b) { LinearLayout.LayoutParams p = new LinearLayout.LayoutParams(w, h); p.setMargins(dp(l), dp(t), dp(r), dp(b)); return p; }
    private int dp(int v) { return (int) (v * getResources().getDisplayMetrics().density + 0.5f); }
    private static String safe(String v) { return v == null ? "" : v.trim(); }

    @Override protected void onResume() { super.onResume(); if (receiverRegistered) queryPlayerState(); }

    @Override public void onTrimMemory(int level) {
        super.onTrimMemory(level);
        if (images != null) images.trimMemory(level);
    }

    @Override public void onLowMemory() {
        super.onLowMemory();
        if (images != null) images.trimMemory(android.content.ComponentCallbacks2.TRIM_MEMORY_COMPLETE);
    }

    @Override protected void onDestroy() {
        destroyed = true;
        healthGeneration.incrementAndGet(); filterGeneration.incrementAndGet(); catalogGeneration.incrementAndGet();
        ui.removeCallbacksAndMessages(null);
        searchRunnable = null;
        uninstallBackHandler();
        if (receiverRegistered) { try { unregisterReceiver(playerReceiver); } catch (Throwable ignored) { } }
        if (repository != null) repository.shutdown();
        filterWorker.shutdownNow(); ioWorker.shutdownNow();
        if (images != null) images.close();
        super.onDestroy();
    }
}

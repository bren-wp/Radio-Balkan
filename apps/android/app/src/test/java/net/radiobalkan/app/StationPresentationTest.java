package net.radiobalkan.app;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import org.junit.Test;
import java.util.Arrays;
import java.util.List;

public class StationPresentationTest {
    @Test public void diasporaDescriptionUsesPublicMetadataOnly() {
        RadioStation station = new RadioStation();
        station.name = "Radio Dijaspora";
        station.country = "Germany";
        station.countryCode = RadioRepository.DIASPORA_CODE;
        station.sourceCountryCode = "DE";
        station.tags = "hits,dijaspora,pop";
        station.language = "Croatian";
        station.codec = "mp3";
        station.bitrate = 128;
        station.urlResolved = "https://secret-stream.example/live";
        station.homepage = "https://admin-homepage.example/";

        String description = StationPresentation.description(station);
        String details = StationPresentation.publicDetails(station);

        assertTrue(description.contains("Dijaspora · Germany"));
        assertTrue(description.contains("Hits") || description.contains("hits"));
        assertTrue(details.contains("MP3"));
        assertTrue(details.contains("128 kbps"));
        assertFalse(description.contains(station.urlResolved));
        assertFalse(description.contains(station.homepage));
        assertFalse(details.contains(station.urlResolved));
        assertFalse(details.contains(station.homepage));
    }

    @Test public void syntheticDiasporaMarkerIsNotPresentedAsGenre() {
        assertEquals("pop", StationPresentation.firstUsefulTag("dijaspora,pop,hits"));
        assertEquals("", StationPresentation.firstUsefulTag("dijaspora"));
    }

    @Test public void similarStationsPreferSameAreaThenSharedGenre() {
        RadioStation current = station("current", "HR", "", "pop,hits", 50);
        RadioStation sameArea = station("same-area", "HR", "", "news", 10);
        RadioStation sharedGenre = station("shared-genre", "RS", "", "pop", 500);
        RadioStation unrelated = station("unrelated", "RS", "", "news", 999);
        RadioStation sameSourceDiaspora = station("diaspora-a", RadioRepository.DIASPORA_CODE, "DE", "folk,dijaspora", 20);
        RadioStation diasporaPeer = station("diaspora-b", RadioRepository.DIASPORA_CODE, "DE", "news,dijaspora", 15);

        List<RadioStation> regional = StationPresentation.similarStations(
                Arrays.asList(current, sharedGenre, unrelated, sameArea), current, 3);
        assertEquals(2, regional.size());
        assertEquals("same-area", regional.get(0).stationUuid);
        assertEquals("shared-genre", regional.get(1).stationUuid);

        List<RadioStation> diaspora = StationPresentation.similarStations(
                Arrays.asList(sameSourceDiaspora, diasporaPeer, unrelated), sameSourceDiaspora, 3);
        assertEquals(1, diaspora.size());
        assertEquals("diaspora-b", diaspora.get(0).stationUuid);
    }

    private static RadioStation station(String id, String code, String sourceCode, String tags, int votes) {
        RadioStation value = new RadioStation();
        value.stationUuid = id;
        value.name = id;
        value.countryCode = code;
        value.sourceCountryCode = sourceCode;
        value.tags = tags;
        value.votes = votes;
        return value;
    }

    @Test public void supplementalAreaLabelsRemainDistinct() {
        RadioStation diaspora = new RadioStation();
        diaspora.countryCode = RadioRepository.DIASPORA_CODE;
        diaspora.country = "Austria";
        assertEquals("Dijaspora · Austria", StationPresentation.area(diaspora));

        RadioStation foreign = new RadioStation();
        foreign.countryCode = RadioRepository.FOREIGN_CODE;
        foreign.country = "United States";
        assertEquals("Strano · United States", StationPresentation.area(foreign));
    }
}

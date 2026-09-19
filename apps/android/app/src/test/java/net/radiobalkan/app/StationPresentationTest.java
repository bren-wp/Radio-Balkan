package net.radiobalkan.app;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import org.junit.Test;

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

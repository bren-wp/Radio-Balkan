package net.radiobalkan.app;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import org.junit.Test;

public final class RadioRepositoryTest {
    @Test
    public void exposesForeignGroupWithoutTreatingArbitraryCountriesAsSupported() {
        assertTrue(RadioRepository.isSupportedCountry("HR"));
        assertTrue(RadioRepository.isSupportedCountry("BG"));
        assertEquals("Bugarska", RadioRepository.countryName("BG"));
        assertTrue(RadioRepository.isSupportedCountry(RadioRepository.FOREIGN_CODE));
        assertTrue(RadioRepository.isSupportedCountry(RadioRepository.DIASPORA_CODE));
        assertFalse(RadioRepository.isSupportedCountry("US"));
        assertFalse(RadioRepository.isSupportedCountry("GB"));
        assertEquals("Strano", RadioRepository.countryName(RadioRepository.FOREIGN_CODE));
        assertEquals("Dijaspora", RadioRepository.countryName(RadioRepository.DIASPORA_CODE));
    }

    @Test
    public void supplementalGroupsHaveStableBrowseOrder() {
        String[] bulgaria = RadioRepository.COUNTRIES[RadioRepository.COUNTRIES.length - 3];
        String[] diaspora = RadioRepository.COUNTRIES[RadioRepository.COUNTRIES.length - 2];
        String[] foreign = RadioRepository.COUNTRIES[RadioRepository.COUNTRIES.length - 1];
        assertEquals("BG", bulgaria[0]);
        assertEquals("Bugarska", bulgaria[1]);
        assertEquals(RadioRepository.DIASPORA_CODE, diaspora[0]);
        assertEquals("Dijaspora", diaspora[1]);
        assertEquals(RadioRepository.FOREIGN_CODE, foreign[0]);
        assertEquals("Strano", foreign[1]);
    }
}

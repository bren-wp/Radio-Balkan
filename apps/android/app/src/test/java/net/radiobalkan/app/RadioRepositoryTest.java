package net.radiobalkan.app;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import org.junit.Test;

public final class RadioRepositoryTest {
    @Test
    public void exposesForeignGroupWithoutTreatingArbitraryCountriesAsSupported() {
        assertTrue(RadioRepository.isSupportedCountry("HR"));
        assertTrue(RadioRepository.isSupportedCountry(RadioRepository.FOREIGN_CODE));
        assertFalse(RadioRepository.isSupportedCountry("US"));
        assertFalse(RadioRepository.isSupportedCountry("GB"));
        assertEquals("Strano", RadioRepository.countryName(RadioRepository.FOREIGN_CODE));
    }

    @Test
    public void foreignGroupIsLastBrowseCategory() {
        String[] last = RadioRepository.COUNTRIES[RadioRepository.COUNTRIES.length - 1];
        assertEquals(RadioRepository.FOREIGN_CODE, last[0]);
        assertEquals("Strano", last[1]);
    }
}

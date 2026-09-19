package net.radiobalkan.app;

import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;
import org.junit.Test;

public class AdminAuthTest {
    @Test public void acceptsOnlyConfiguredAdministratorCredentials() {
        assertTrue(AdminAuth.matches("brendigo", "brendigo" + "2025"));
        assertTrue(AdminAuth.matches(" BRENDIGO ", "brendigo" + "2025"));
        assertFalse(AdminAuth.matches("user", "brendigo" + "2025"));
        assertFalse(AdminAuth.matches("brendigo", "wrong"));
        assertFalse(AdminAuth.matches(null, "brendigo" + "2025"));
    }
}

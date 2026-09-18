package net.radiobalkan.app;

import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import org.junit.Test;
import java.net.InetAddress;

public final class StreamResolverTest {
    @Test
    public void rejectsPrivateCredentialedAndMetadataTargets() {
        String[] rejected = {
                "",
                "file:///tmp/radio",
                "http://localhost/live",
                "http://localhost./live",
                "http://radio.local./live",
                "http://127.0.0.1/live",
                "http://10.0.0.4/live",
                "http://172.16.1.2/live",
                "http://192.168.1.10/live",
                "http://169.254.169.254/latest/meta-data/",
                "http://100.64.0.1/live",
                "http://[::1]/live",
                "http://[fc00::1]/live",
                "http://[fe80::1]/live",
                "https://metadata.google.internal/computeMetadata/v1/",
                "https://metadata.google.internal./computeMetadata/v1/",
                "https://user:pass@example.com/live"
        };
        for (String raw : rejected) {
            assertFalse("expected unsafe URL to be rejected: " + raw, StreamResolver.isSafeHttp(raw));
        }
    }

    @Test
    public void acceptsNormalPublicHttpStreams() {
        assertTrue(StreamResolver.isSafeHttp("https://example.com/live.mp3"));
        assertTrue(StreamResolver.isSafeHttp("http://stream.example.org:8000/radio"));
    }

    @Test
    public void httpSchemeCheckDoesNotAcceptLookalikes() {
        assertTrue(StreamResolver.isHttp("https://example.com/live"));
        assertTrue(StreamResolver.isHttp("http://example.com/live"));
        assertFalse(StreamResolver.isHttp("httpsx://example.com/live"));
        assertFalse(StreamResolver.isHttp("ftp://example.com/live"));
    }

    @Test
    public void resolvedAddressSetRejectsAnyPrivateOrLocalTarget() throws Exception {
        assertFalse(StreamResolver.allAddressesSafe(new InetAddress[] {
                InetAddress.getByAddress(new byte[] {8, 8, 8, 8}),
                InetAddress.getByAddress(new byte[] {127, 0, 0, 1})
        }));
        assertFalse(StreamResolver.allAddressesSafe(new InetAddress[] {
                InetAddress.getByAddress(new byte[] {(byte) 169, (byte) 254, (byte) 169, (byte) 254})
        }));
        assertTrue(StreamResolver.allAddressesSafe(new InetAddress[] {
                InetAddress.getByAddress(new byte[] {8, 8, 8, 8}),
                InetAddress.getByAddress(new byte[] {1, 1, 1, 1})
        }));
    }

    @Test
    public void foreignRepairAcceptsOnlyNonBalkanCatalogCountry() {
        assertTrue(StreamResolver.matchesRefreshCountry(RadioRepository.FOREIGN_CODE, "US"));
        assertTrue(StreamResolver.matchesRefreshCountry(RadioRepository.FOREIGN_CODE, "GB"));
        assertFalse(StreamResolver.matchesRefreshCountry(RadioRepository.FOREIGN_CODE, "HR"));
        assertFalse(StreamResolver.matchesRefreshCountry(RadioRepository.FOREIGN_CODE, "RS"));
        assertFalse(StreamResolver.matchesRefreshCountry(RadioRepository.FOREIGN_CODE, ""));
        assertTrue(StreamResolver.matchesRefreshCountry("HR", "HR"));
        assertFalse(StreamResolver.matchesRefreshCountry("HR", "US"));
    }
}

package io.github.vrushankpatel.bluelink.firebase;

import javax.crypto.Cipher;
import javax.crypto.SecretKey;
import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.SecretKeySpec;
import java.nio.ByteBuffer;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.util.Base64;

/**
 * AES-256-GCM encryption/decryption.
 *
 * Key derivation : SHA-256 of the room ID (same scheme as the Go version).
 * Nonce          : 12 random bytes from SecureRandom prepended to the ciphertext
 *                  (improvement over the Go version's deterministic nonce).
 *
 * Wire format (Base64): [ 12-byte nonce | ciphertext+tag ]
 */
final class Crypto {

    private static final String ALGORITHM  = "AES/GCM/NoPadding";
    private static final int    NONCE_LEN  = 12;   // 96-bit nonce — GCM standard
    private static final int    TAG_BITS   = 128;  // 128-bit authentication tag

    private static final SecureRandom RANDOM = new SecureRandom();

    private Crypto() {}

    static String encrypt(String plaintext, String roomId) throws Exception {
        SecretKey key   = deriveKey(roomId);
        byte[]    nonce = new byte[NONCE_LEN];
        RANDOM.nextBytes(nonce);

        Cipher cipher = Cipher.getInstance(ALGORITHM);
        cipher.init(Cipher.ENCRYPT_MODE, key, new GCMParameterSpec(TAG_BITS, nonce));
        byte[] ciphertext = cipher.doFinal(plaintext.getBytes("UTF-8"));

        // Prepend nonce to ciphertext
        ByteBuffer buf = ByteBuffer.allocate(NONCE_LEN + ciphertext.length);
        buf.put(nonce);
        buf.put(ciphertext);
        return Base64.getEncoder().encodeToString(buf.array());
    }

    static String decrypt(String encoded, String roomId) throws Exception {
        byte[]     raw    = Base64.getDecoder().decode(encoded);
        ByteBuffer buf    = ByteBuffer.wrap(raw);

        byte[] nonce      = new byte[NONCE_LEN];
        byte[] ciphertext = new byte[raw.length - NONCE_LEN];
        buf.get(nonce);
        buf.get(ciphertext);

        SecretKey key    = deriveKey(roomId);
        Cipher    cipher = Cipher.getInstance(ALGORITHM);
        cipher.init(Cipher.DECRYPT_MODE, key, new GCMParameterSpec(TAG_BITS, nonce));
        byte[] plaintext = cipher.doFinal(ciphertext);
        return new String(plaintext, "UTF-8");
    }

    private static SecretKey deriveKey(String roomId) throws Exception {
        MessageDigest sha = MessageDigest.getInstance("SHA-256");
        byte[]        raw = sha.digest(roomId.getBytes("UTF-8"));
        return new SecretKeySpec(raw, "AES");
    }
}

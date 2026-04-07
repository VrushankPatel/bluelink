package io.github.vrushankpatel.bluelink.firebase;

/**
 * Represents a single chat message stored in Firebase.
 */
public class Message {

    private String sender;
    private String senderId;
    private String color;
    private String text;
    private long   timestamp;

    public Message() {}

    public Message(String sender, String senderId, String color, String text, long timestamp) {
        this.sender    = sender;
        this.senderId  = senderId;
        this.color     = color;
        this.text      = text;
        this.timestamp = timestamp;
    }

    public String getSender()    { return sender; }
    public String getSenderId()  { return senderId; }
    public String getColor()     { return color; }
    public String getText()      { return text; }
    public long   getTimestamp() { return timestamp; }

    public void setText(String text) { this.text = text; }
}

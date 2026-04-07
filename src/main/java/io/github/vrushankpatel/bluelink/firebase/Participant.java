package io.github.vrushankpatel.bluelink.firebase;

/**
 * Represents a participant in a chat room.
 */
public class Participant {

    private String name;
    private String color;
    private long   lastActive;

    public Participant() {}

    public Participant(String name, String color, long lastActive) {
        this.name       = name;
        this.color      = color;
        this.lastActive = lastActive;
    }

    public String getName()       { return name; }
    public String getColor()      { return color; }
    public long   getLastActive() { return lastActive; }
}

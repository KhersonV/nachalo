# Full match map in the client payload

GameService currently sends the complete `map` array in the authenticated match response and again on reconnect. The client receives terrain and cell object data for unexplored coordinates. Classic and 3D fog hide that information in the interface, but a player can inspect the network response or client memory.

The 3D test renderer reads the existing Redux match snapshot. It does not request or expose additional server data. Its display adapter removes unknown cells and hides dynamic objects outside current visibility before creating visible sprites.

Preventing inspection of unexplored map data requires a separate server protocol change: send only authorized terrain and object information, track exploration per player or team on the server, and update it as visibility changes. That work must preserve reconnect behavior and be verified across teammates, spectators, deaths, and completed matches. It is outside the graphics integration.

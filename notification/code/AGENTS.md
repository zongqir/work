# Code Notes

- Prefer the smallest implementation that makes the behavior obvious.
- If a feature can be done with one simple model and one loader, do not add extension points, wrapper interfaces, or staged abstractions.
- Keep capability JSON per `message_type` and keep validation close to the data it validates.

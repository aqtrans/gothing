#20150205 
- Migrated from Bootstrap to Foundation, mainly to get rid of the baggage taken on since this Bootstrap theme was ported from a Jekyll theme originally...lots of excess

#20150205
 - Added an official changelog, but most other changes can be found in the git commit log

#20150207 
- Replaced Negroni/Gorilla-Mux combination with Goji, which comes along with the handy (but difficult to grasp for me) web.Context feature, replacing my old GuardPath pseudo-middleware.

#20150212 
- Replaced goji.Serve() call with my own replicant, to fully use my custom logger middleware.
- Added a /big/ route that uses gifsicle (after finding ImageMagick loves memory) to embiggen gifs
    - Haven't made this publicly linked on any pages yet, as I want to possibly set the embiggened gifs up to be automatically resized in a goroutine after uploading to save on resources
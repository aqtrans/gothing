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

#20150223
- Sanitized all private info for public release I hope
- Config file integration! 
    - conf.json in the same dir as the binary is parsed for all necessary info to get the app up and running on other domains
- Foregone golang-assisted sanitizing or HTML escaping for pastes in favor of simple strings.replace() functions
    - Every sanitize feature I could find demanded escaping all '<', and '>', which makes the pastebin useless for simple bash scripts :(
    - This is very basic and likely easy to work around, as I am only turning '<script>' into '< script >' at the moment, just to stop javascript XSS attacks

#20150228
- Renamed from TKOT to GoBanana, to go along with the Go[whatever] naming convention, and I wasn't too fond of TKOT in the first place
- Moved primary dev source from Github to my personal Gitlab install with infrequent mirroring to Github when I feel like it 

#20150502
- Renamed to GoThing
- Major revamp of most API calls
    - Now properly taking advantage of javascript (jquery) to run the calls and pipe the results to a nice alert banner at the top of pages
    
#20150509
- Replaced Goji with Gorilla/Mux...back to where we began!
    - Due to it's superb matching capabilities, allowing me to properly capture my *.es.gy wildcard short URLs
    - I was also not using Goji's context feature whatsoever once I switched to my homebrewed cookie-based auth, negating it's original draw for me
     
#20150726
- Replaced plaintext login with LDAP, powered by [github.com/mavricknz/ldap]()
    - I need to work on pulling this into a separate library and reuse elsewhere
    - Along with that, should make the backend configurable. Plaintext or LDAP for now.

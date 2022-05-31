package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/crazyinfin8/mpg-go"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Game struct {
	player     *mpg.Player
	videoFrame *image.RGBA
	videoImage *ebiten.Image
	ctx        *audio.Context
	p          *audio.Player
	ptime      time.Time

	playing   bool
	loop      bool
	debug     bool
	scrubbing bool

	playheadPos   float64
	playheadColor color.Color

	// TODO: try implementing preview frames when hovering over the playhead
	pmouseX      int
	previewFrame *image.RGBA
	previewImage *ebiten.Image
	previewPos   *ebiten.DrawImageOptions
}

func main() {
	defer func(err interface{}) {
		if err != nil {
			fmt.Printf("Recovered from panic:\n\n%#v", err)
		}
	}(recover())

	var (
		videoPath string
		err       error
		printHelp bool
	)
	g := new(Game)

	flag.BoolVar(&g.loop, "l", false, "Loop video playback")
	flag.BoolVar(&g.loop, "loop", false, "Loop video playback")
	flag.StringVar(&videoPath, "p", "", "Path to MPG file")
	flag.StringVar(&videoPath, "path", "", "Path to MPG file")
	flag.BoolVar(&g.debug, "debug", false, "prints statistics to the screen")
	flag.BoolVar(&g.debug, "d", false, "prints statistics to the screen")
	flag.BoolVar(&printHelp, "h", false, "Prints this help info and exits")
	flag.BoolVar(&printHelp, "help", false, "Prints this help info and exits")
	flag.Parse()
	println("MPG-Go")
	println("======")
	if videoPath == "" || printHelp {
		flag.PrintDefaults()
		return
	}

	g.player, err = mpg.NewPlayerFromFilename(videoPath)
	if err != nil {
		println("Error:", err.Error())
		return
	}
	defer g.player.Close()

	// Setup and display generic stats
	fmt.Printf(
		"Create player successful\n"+
			"\tDuration: %s\n\n",
		g.player.Duration().Round(time.Second/100),
	)
	if g.loop {
		g.player.SetLoop(true)
	}

	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	// Setup and display video stats
	vStreams := g.player.NumVideoStreams()
	if g.player.HasVideo() {
		width, height := g.player.Width(), g.player.Height()
		framerate := g.player.FrameRate()
		fmt.Printf(
			"Video detected\n"+
				"\tNumChannels: %d\n"+
				"\tWidth:       %d\n"+
				"\tHeight:      %d\n"+
				"\tFrameRate:   %00.2f\n\n",
			vStreams, width, height, framerate,
		)
		ebiten.SetWindowSize(width, height)

		g.videoFrame = image.NewRGBA(image.Rectangle{
			image.Point{},
			image.Point{width, height},
		})

		// DrawRGBAPixels does not set alpha channels, so we set them here.
		mpg.SetAlpha(0xFF, g.videoFrame.Pix)

		g.videoImage = ebiten.NewImageFromImage(g.videoFrame)

		g.previewFrame = image.NewRGBA(image.Rectangle{
			image.Point{},
			image.Point{width, height},
		})
		// same for DrawFrameAsRGBAPixels.
		mpg.SetAlpha(0xFF, g.previewFrame.Pix)

		g.previewImage = ebiten.NewImageFromImage(g.previewFrame)
		g.previewPos = &ebiten.DrawImageOptions{}
	} else {
		fmt.Printf(
			"No video detected\n"+
				"\tNumChannels: %d\n\n",
			vStreams,
		)
	}

	// Setup and display Audio stats.
	aStreams := g.player.NumAudioStreams()
	if g.player.HasAudio() {
		samplerate := g.player.SampleRate()
		fmt.Printf(
			"Audio detected\n"+
				"\tNumChannels: %d\n"+
				"\tSampleRate:  %d\n\n",
			aStreams, samplerate,
		)
		g.ctx = audio.NewContext(samplerate)
		g.p, _ = g.ctx.NewPlayer(g.player)
		g.p.SetBufferSize(g.player.AudioLeadTime())
		g.p.Play()
	} else {
		fmt.Printf(
			"No audio detected\n"+
				"\tNumChannels: %d\n\n",
			aStreams,
		)
	}
	// g.p.Clo

	fmt.Print(
		"Controls:\n" +
			"\t[space] pause/play stream\n" +
			"\t[esc] stop the stream and exits\n" +
			"\t[<-] seek back 15 sec\n" +
			"\t[->] seek forward 15 sec\n\n")

	// Start stream and runs ebiten.
	g.Play()
	if err := ebiten.RunGame(g); err != nil && err != done {
		panic(err.Error())
	} else if err == done {
		fmt.Println("\nDone!")
	}
}

func (g *Game) Update() error {
	// Exit the player if the stream is finished or the escape key is pressed.
	if g.player.Finished() || inpututil.IsKeyJustReleased(ebiten.KeyEscape) {
		return done
	}

	// Space key to play or pause the stream.
	if inpututil.IsKeyJustReleased(ebiten.KeySpace) {
		g.TogglePlaying()
	}

	// seek 15 seconds forward and backward using arrow keys.
	if inpututil.IsKeyJustReleased(ebiten.KeyRight) {
		g.player.Seek(g.player.Time()+15*time.Second, true)
	} else if inpututil.IsKeyJustReleased(ebiten.KeyLeft) {
		g.player.Seek(g.player.Time()-15*time.Second, true)
	}

	// Process stream, progress bar, and allow scrubbing to specific periods of the stream.
	width, height := float64(g.player.Width()), float64(g.player.Height())
	mouseX, mouseY := ebiten.CursorPosition()
	g.playheadColor = color.Alpha{0x10}
	if float64(mouseY) < height && float64(mouseY) > height-15 {
		g.playheadColor = color.RGBA{0xFF, 0x00, 0x00, 0xFF}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.scrubbing = true
			g.Pause()
		}
	}
	if g.scrubbing {
		g.playheadPos = float64(mouseX) / width
		if mouseX != g.pmouseX {
			// If mouse moved, grab a preview frame at specific time and draw it above the playhead.
			ok := g.player.ReadRGBAAt(g.previewFrame.Pix, time.Duration(float64(g.player.Duration())*float64(mouseX)/width), false)
			if ok {
				g.previewImage.ReplacePixels(g.previewFrame.Pix)
				g.previewPos.GeoM.Reset()
				g.previewPos.GeoM.Scale(0.25, 0.25)
				tx, ty := float64(mouseX)-width/8, height/4*3-15
				tx = math.Min(math.Max(0, tx), width/4*3)
				g.previewPos.GeoM.Translate(tx, ty)
			}
			g.pmouseX = mouseX
		}
		if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
			// Scrub to the playhead time and resume playback
			g.player.Seek(time.Duration(float64(g.player.Duration())*g.playheadPos), true)
			g.Play()
			g.scrubbing = false
		}
	} else {
		g.playheadPos = g.player.Time().Seconds() / g.player.Duration().Seconds()
		if g.playing {
			// Process and decode stream and sets playhead.
			now := time.Now()
			g.player.Decode(now.Sub(g.ptime))
			g.ptime = now
		}
	}

	// Render frame to image.
	if g.player.HasNewFrame() {
		g.player.ReadRGBA(g.videoFrame.Pix)
		g.videoImage.ReplacePixels(g.videoFrame.Pix)
	}

	fmt.Printf("Progress: %s / %s\t\t\r",
		g.player.Time().Round(time.Second),
		g.player.Duration().Round(time.Second),
	)
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Draws video if it exists.
	if g.videoImage != nil {
		screen.DrawImage(g.videoImage, nil)
		if g.scrubbing && g.previewImage != nil {
			screen.DrawImage(g.previewImage, g.previewPos)
		}
	} else {
		screen.Clear()
	}

	// Prints some information about performance and progress.
	if g.debug {
		ebitenutil.DebugPrint(screen,
			fmt.Sprintf(
				"Debug Info\n"+
					"    FPS:          %0.2f\n"+
					"    TPS:          %0.2f\n"+
					"    Current Time: %s\n",
				ebiten.CurrentFPS(),
				ebiten.CurrentTPS(),
				g.player.Time().Round(time.Second/100),
			),
		)
	}

	// Draws playhead and preview image.
	width, height := float64(g.player.Width()), float64(g.player.Height())
	ebitenutil.DrawRect(screen, 0, height-15, width*g.playheadPos, 15, g.playheadColor)

}

func (g *Game) Layout(width, height int) (int, int) {
	return g.player.Width(), g.player.Height()
}

var done = Done{}

// Returned when Ebiten is finished.
type Done struct{}

func (Done) Error() string { return "Done" }

// TogglePlaying toggles whether the stream is playing or pausing.
func (g *Game) TogglePlaying() {
	if g.playing {
		g.Pause()
	} else {
		g.Play()
	}
}

// Play the stream.
func (g *Game) Play() {
	if g.playing {
		return
	}
	g.playing = true
	g.ptime = time.Now()
}

// Pause the stream.
func (g *Game) Pause() {
	if !g.playing {
		return
	}
	g.playing = false
	g.player.ClearAudioBuffer()
}

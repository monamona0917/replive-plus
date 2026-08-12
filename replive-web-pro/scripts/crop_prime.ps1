Add-Type -AssemblyName System.Drawing

$srcPath = "C:\Users\Fu\.gemini\antigravity\brain\1cb2791b-b636-419f-b48a-24a9b8d472dd\.user_uploaded\media_1786418090823.jpg"
$img = [System.Drawing.Bitmap]::FromFile($srcPath)

$w = $img.Width
$h = $img.Height
Write-Host "Image size: $w x $h"

# Find bounds of golden pixels
$minX = $w
$maxX = 0
$minY = $h
$maxY = 0

for ($y = 0; $y -lt $h; $y++) {
    for ($x = 0; $x -lt ($w / 2); $x++) {
        $pixel = $img.GetPixel($x, $y)
        # Check gold/amber color: high R, medium-high G, low B
        if ($pixel.R -gt 170 -and $pixel.G -gt 140 -and $pixel.B -lt 120) {
            if ($x -lt $minX) { $minX = $x }
            if ($x -gt $maxX) { $maxX = $x }
            if ($y -lt $minY) { $minY = $y }
            if ($y -gt $maxY) { $maxY = $y }
        }
    }
}

Write-Host "Sparkle bounds: ($minX, $minY) to ($maxX, $maxY), width: $($maxX - $minX + 1), height: $($maxY - $minY + 1)"

$pad = 4
$cropX = [Math]::Max(0, $minX - $pad)
$cropY = [Math]::Max(0, $minY - $pad)
$cropW = [Math]::Min($w - $cropX, ($maxX - $minX + 1) + $pad * 2)
$cropH = [Math]::Min($h - $cropY, ($maxY - $minY + 1) + $pad * 2)

$rect = New-Object System.Drawing.Rectangle($cropX, $cropY, $cropW, $cropH)
$cropped = $img.Clone($rect, $img.PixelFormat)

$outDir = "D:\Tencentt\Tencent Files\1528760842\文件\MobileFile\nsy_chat_live-master\replive-web-pro\public"
$outPath = Join-Path $outDir "prime-icon-original.png"
$cropped.Save($outPath, [System.Drawing.Imaging.ImageFormat]::Png)
Write-Host "Saved original crop to $outPath"

# Also create high-res transparent PNG
$hdW = 512
$hdH = [int](512 * ($cropH / $cropW))
$hdBmp = New-Object System.Drawing.Bitmap($hdW, $hdH)
$g = [System.Drawing.Graphics]::FromImage($hdBmp)
$g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
$g.DrawImage($cropped, 0, 0, $hdW, $hdH)
$g.Dispose()

$hdPath = Join-Path $outDir "prime-icon-hd.png"
$hdBmp.Save($hdPath, [System.Drawing.Imaging.ImageFormat]::Png)
Write-Host "Saved HD PNG to $hdPath"

$cropped.Dispose()
$img.Dispose()
$hdBmp.Dispose()

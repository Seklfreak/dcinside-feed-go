<?php

error_reporting(E_ALL);
ini_set('display_errors', 1);

// source: http://nl3.php.net/manual/en/function.mime-content-type.php#87856
$MimeTypes = array( 
    // images
    'png' => 'image/png',
    'jpe' => 'image/jpeg',
    'jpeg' => 'image/jpeg',
    'jpg' => 'image/jpeg',
    'gif' => 'image/gif',
    'bmp' => 'image/bmp',
    'ico' => 'image/vnd.microsoft.icon',
    'tiff' => 'image/tiff',
    'tif' => 'image/tiff',
    'svg' => 'image/svg+xml',
    'svgz' => 'image/svg+xml'
);

define('INI_FILE_LOCATION', 'C:\Users\user\Documents\dcinside-feed-go\config.ini');

$configIniArray = parse_ini_file(INI_FILE_LOCATION);

if (isset($configIniArray['safe links file']) && $configIniArray['safe links file'] != null
    && isset($_GET['url']) && $_GET['url'] != null) {
    $safeLinksHandle = fopen($configIniArray['safe links file'], 'r');
    $linkIsSafe = false;
    while (($buffer = fgets($safeLinksHandle)) !== false) {
        if (strpos($buffer, md5($_GET['url'])) !== false) {
            $linkIsSafe = true;
            break;
        }
    }
    fclose($safeLinksHandle);

    if ($linkIsSafe == false) {
        header("HTTP/1.1 403 Forbidden");
    } else {
        $ch = curl_init();
        curl_setopt($ch, CURLOPT_URL, $_GET['url']);
        curl_setopt($ch, CURLOPT_USERAGENT,
            'Mozilla/5.0 (Windows; U; Windows NT 5.1; en-US; rv:1.8.1.13) Gecko/20080311 Firefox/2.0.0.13');
        curl_setopt($ch, CURLOPT_URL, $_GET['url']);
        curl_setopt($ch, CURLOPT_REFERER, 'http://gall.dcinside.com/');
        curl_setopt($ch, CURLOPT_TIMEOUT, 60);
        curl_setopt($ch, CURLOPT_HEADER, true);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, 1);
        if (isset($configIniArray['socks4 proxy']) && $configIniArray['socks4 proxy'] != null) {
            curl_setopt($ch, CURLOPT_PROXY, $configIniArray["socks4 proxy"]);
            curl_setopt($ch, CURLOPT_PROXYTYPE, 6); // CURLPROXY_SOCKS4A
        }
        $response = curl_exec($ch);

        if ($response != null) {
            if (curl_getinfo($ch, CURLINFO_HTTP_CODE) == 200) {
                $file_array = explode("\n\r", $response, 2);
                $header_array = explode("\n", $file_array[0]);
                foreach($header_array as $header_value) {
                $header_pieces = explode(':', $header_value);
                    if(count($header_pieces) == 2) {
                        $headers[$header_pieces[0]] = trim($header_pieces[1]);
                    }
                }
                $reDispo = '/^Content-Disposition: .*?filename=(?<f>[^\s]+|\x22[^\x22]+\x22)\x3B?.*$/m';
                if (preg_match($reDispo, $response, $mDispo)) {
                    $filename = trim($mDispo['f'],' ";');
                }

                $contentType = "application/octet-stream";
                if (isset($MimeTypes[strtolower(pathinfo($filename, PATHINFO_EXTENSION))])) {
                    $contentType = $MimeTypes[strtolower(pathinfo($filename, PATHINFO_EXTENSION))];
                }

                header('Content-Type: '.$contentType);
                header('Content-Disposition: inline; filename='.$filename);
                header('Cache-Control: max-age=2678400');
                echo substr($file_array[1], 1);
            }
        }

        curl_close($ch);
    }
}

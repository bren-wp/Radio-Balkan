plugins {
    id("com.android.application")
}

val releaseKeystorePath = System.getenv("RADIO_BALKAN_KEYSTORE")
val releaseStorePassword = System.getenv("RADIO_BALKAN_STORE_PASSWORD")
val releaseKeyAlias = System.getenv("RADIO_BALKAN_KEY_ALIAS")
val releaseKeyPassword = System.getenv("RADIO_BALKAN_KEY_PASSWORD")
val hasReleaseSigning = !releaseKeystorePath.isNullOrBlank() && !releaseStorePassword.isNullOrBlank() && !releaseKeyAlias.isNullOrBlank() && !releaseKeyPassword.isNullOrBlank()

android {
    namespace = "net.radiobalkan.app"
    compileSdk = 36

    defaultConfig {
        applicationId = "net.radiobalkan.app"
        minSdk = 26
        targetSdk = 36
        versionCode = 21
        versionName = "0.0.21"
    }

    signingConfigs {
        create("release") {
            if (hasReleaseSigning) {
                storeFile = file(releaseKeystorePath!!)
                storePassword = releaseStorePassword
                keyAlias = releaseKeyAlias
                keyPassword = releaseKeyPassword
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            if (hasReleaseSigning) signingConfig = signingConfigs.getByName("release")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    lint {
        abortOnError = true
        checkReleaseBuilds = true
    }
}

dependencies {
    testImplementation("junit:junit:4.13.2")
}

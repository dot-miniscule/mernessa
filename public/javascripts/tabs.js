/* ====================== MAINPAGE  ===================== */
/* THIS IS THE JAVASCRIPT FOR THE MAIN PAGE */

//----------- TAB SELECTION ------------//
$(function() {
  $('.tab-panels .tabs li').on('click', function() {

    //On mouse click find the next element of tab panels
    //find the closes active one and remove that class
    //set the current panel to active
    var $panel = $(this).closest('.tab-panels');
    $panel.find('.tabs li.active').removeClass('active');
    $(this).addClass('active');

    //find which panel to show
    //The related panel is saved in the list item as an attribute
    var panelToShow = $(this).attr('rel');

    //Hide the current panel
    $panel.find('.panel.active').slideUp(400, showNextPanel);

    //show next panel
    function showNextPanel() {
      $('.panel.active').removeClass('active');

      $('#'+panelToShow).slideDown(400, function() {
        $(this).addClass('active');
      });
    }

    var subpage = window.location.href.slice(window.location.href.lastIndexOf('/'));
    var url = removeQuery('tab', subpage)
    url = addQuery('tab', panelToShow, url);
    window.history.pushState('', document.title, url);
  });
});

//---------- SET ACTIVE TAB --------------//
function setActiveTab(sourceURL) {
  var queries = sourceURL.slice(window.location.href.indexOf('?')+1).split('&');
  var param;
  for (var i = 0; i < queries.length; i++) {
    var urlparam = queries[i].split('=');
    if (urlparam[0] == 'tab') {
      param = urlparam[1];
      break;
    }
  }
  $('li[rel="'+param+'"]').addClass('active');
  $('#' + param).addClass('active');
}

//---------- CLEAR SELECTION BUTTON -------------- //
$(function() {
  $('#clearButton').on('click', function() {
    $('.no_change_radios').prop('checked', true);
  });
});

//---------- SUBMIT BUTTON RELOAD PAGE ----------- //
function checkDB(buttonPressed, updateTime) {
  $.post('/dbUpdated?time='+updateTime, function( data ) {
    var titles_selector;
    if (buttonPressed == 'submit') {
      titles_selector = $('#new_state_menu option[value!="no_change"]:selected').parent().parent().siblings('#question').children('#question_title');
    } else if (buttonPressed != '') {
      titles_selector = $('#one-click.clicked').parent().siblings('#question').children('#question_title');
      $('#new_state_menu').val('no_change');
      $('#one-click.clicked').parent().parent().children('td').children('select').val(buttonPressed);
    }

    dbChanged = data.indexOf('Updated: true') > -1;
    if (dbChanged) {
      // Getting the values of the checked radios and saving them as an array
      titles = titles_selector.map(function() {
        return $(this).text();
      });

      // Writing the text to display in the confirm dialog box
      confirm_text = 'Database has been updated\nChanges:\n';
      for (i = 0; i < titles.length; i++) {
        if (data.indexOf(titles[i]) > -1) {
          confirm_text += '\t* ' + titles[i] + '\n';
        }
      }

      confirm_text += '\nDo you wish to continue submit?';
      if (confirm_text.indexOf('*') > -1) {
        if (!confirm(confirm_text)) {
          $('form select').val('no_change');
          return;
        }
      }
    }
    $('#stateForm').submit();
  });
}

$(function() {
  $('#stateForm').submit(function() {
    $('#submitButton').prop('value', 'Submitting');
    $('#submitButton').prop('disabled', true);
    document.cookie = 'submitting=true';

    // Intercept form submission and redirect back to the original page
    $.post( '/', $('#stateForm').serialize()).done(function( data ) {
      window.location = window.location.href.split('#')[0];
    });
    return false;
  });
});

//-------- SETTING COOKIES -------//
$(document).ready(function() {
  setCookies();
  clearEmptyQueries();

  var subpage = window.location.href.split('?')[0].slice(window.location.href.lastIndexOf('/'));
  if (window.location.search.indexOf('tab') == -1 &&
      subpage.indexOf('userPage') == -1 && subpage.indexOf('viewTags') == -1 &&
      subpage.indexOf('viewUsers') == -1) {
    var addedPath = subpage + addQuery('tab', 'unanswered', window.location.search);
    window.history.pushState('', document.title, addedPath);
  }
  setActiveTab(window.location.href);
});

//-------- COOKIES --------//
function setCookies() {
  var code = location.search.split('code=')[1];
  if (code !== undefined && code !== '') {
    document.cookie = 'code=' + code;
  }
  removeQuery('code', window.location.href);
  window.history.pushState("", document.title, "");
}

//-------- Removing queries ---------//
function clearEmptyQueries() {
  var urlParts = window.location.href.split('?');
  var newURL = urlParts[0].slice(window.location.href.lastIndexOf('/'));
  var param;
  var params_arr = [];
  var queryString = urlParts[1];

  if (typeof queryString !== 'undefined') {
    params_arr = queryString.split('&');
    for (var i = params_arr.length -1; i >= 0; i--) {
      query = params_arr[i].split('=')[1];
      if (query == '') {
        params_arr.splice(i, 1);
      }
    }
    newURL += '?' + params_arr.join('&');
  }
  window.history.pushState('', document.title, newURL);
}

// returns path including modified query assuming sourceURL does not
// contain the key
function addQuery(key, query, sourceURL) {
  var newURL = sourceURL;
  if (sourceURL.indexOf('?') !== -1) {
    if (sourceURL.split('?')[1] !== '') {
      newURL += '&';
    }
  } else {
    newURL += '?';
  }
<<<<<<< HEAD
}
=======
  newURL += key + '=' + query;
  return newURL;
}

// returns path without query specified
function removeQuery(key, sourceURL) {
  var newURL = sourceURL.split('?')[0];
  var param;
  var params_arr = [];
  var queryString = sourceURL.split('?')[1];

  if (typeof queryString !== 'undefined') {
    params_arr = queryString.split('&');
    for (var i = params_arr.length -1; i >= 0; i--) {
      param = params_arr[i].split('=')[0];
      if (param == key) {
        params_arr.splice(i, 1);
      }
    }
    if (params_arr.length > 0) {
      newURL += '?' + params_arr.join('&');
    }
  }
  return newURL;
}

